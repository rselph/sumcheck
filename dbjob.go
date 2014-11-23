// dbjob.go
package main

import (
	"fmt"
	"io"
	"log"
	"os/user"
	"path/filepath"

	"code.google.com/p/go-sqlite/go1/sqlite3"
)

const batchSize int = 100

type fileDB struct {
	db *sqlite3.Conn

	insert     *sqlite3.Stmt
	batchCount int

	begin  *sqlite3.Stmt
	commit *sqlite3.Stmt

	get     *sqlite3.Stmt
	getArgs sqlite3.NamedArgs
	//row    sqlite3.RowMap
}

func (fdb *fileDB) Close() {
	if fdb.db != nil {
		if fdb.commit != nil {
			fdb.commit.Exec()
		}

		if fdb.insert != nil {
			fdb.insert.Close()
		}
		if fdb.get != nil {
			fdb.get.Close()
		}
		if fdb.begin != nil {
			fdb.begin.Close()
		}
		if fdb.commit != nil {
			fdb.commit.Close()
		}
	}
}

func newDBConnection() (db *sqlite3.Conn, err error) {
	me, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}

	path := filepath.Join(me.HomeDir, ".tmverify.sqlite3")
	db, err = sqlite3.Open(path)

	if err == nil {
		err = db.Exec("CREATE TABLE IF NOT EXISTS FILES (PATH varchar not null, SIZE integer not null, MOD_TIME integer not null, MODE integer not null, CHKSUM integer not null, primary key(PATH));")
	}

	if err == nil {
		err = db.Exec("CREATE INDEX IF NOT EXISTS BYSUM on FILES (CHKSUM, SIZE);")
	}

	//db.BusyTimeout(time.Second)

	return
}

func closeDBConnection(db *sqlite3.Conn) {
	db.Exec("COMMIT TRANSACTION;")
	db.Close()
}

func newFileDB(db *sqlite3.Conn) (fdb *fileDB, err error) {
	fdb = &fileDB{}

	fdb.insert, err = db.Prepare("insert or replace into FILES (path, size, mod_time, mode, chksum) values ($path, $size, $mod_time, $mode, $chksum);")
	if err != nil {
		fmt.Println("Cannot create insert statement.")
		fdb.Close()
		fdb = nil
		return
	}

	fdb.get, err = db.Prepare("select size, mod_time, mode, chksum from FILES where path = $path;")
	if err != nil {
		fmt.Println("Cannot create select statement.")
		fdb.Close()
		return nil, err
	}

	fdb.begin, err = db.Prepare("BEGIN TRANSACTION;")
	if err != nil {
		fmt.Println("Cannot create begin transaction statement.")
		fdb.Close()
		return nil, err
	}

	fdb.commit, err = db.Prepare("COMMIT TRANSACTION;")
	if err != nil {
		fmt.Println("Cannot create commit transaction statement.")
		fdb.Close()
		return nil, err
	}

	fdb.getArgs = make(sqlite3.NamedArgs)
	//fdb.row = make(sqlite3.RowMap)

	fdb.begin.Exec()

	return fdb, nil
}

func (fdb *fileDB) insertOrReplace(f *fileJob) (err error) {
	args := make(sqlite3.NamedArgs)
	args["$path"] = f.Fpath
	args["$size"] = f.Info.Size()
	args["$mod_time"] = f.Info.ModTime().UnixNano()
	args["$mode"] = int64(f.Info.Mode())
	args["$chksum"] = int64(f.Chksum)

	defer fdb.insert.Reset()
	err = fdb.insert.Exec(args)
	if err != nil {
		return
	}

	if fdb.batchCount += 1; fdb.batchCount >= batchSize {
		err = fdb.commit.Exec()
		if err != nil {
			return
		}
		fdb.begin.Exec()
		if err != nil {
			return
		}
		fdb.batchCount = 0
		//fmt.Print("^")
	}
	return
}

func (fdb *fileDB) getForPath(f *fileJob) (row sqlite3.RowMap, err error) {
	defer fdb.get.Reset()
	fdb.getArgs["$path"] = f.Fpath
	err = fdb.get.Query(fdb.getArgs)
	switch {
	case err == io.EOF:
		err = nil

	case err != nil:

	default:
		row = make(sqlite3.RowMap)
		err = fdb.get.Scan(row)
	}

	return
}

func CheckInDB(f *fileJob, fdb *fileDB) {
	if f.Err != nil {
		return
	}

	row, err := fdb.getForPath(f)
	switch {
	case err != nil:
		// Problem with db.  Fail.
		log.Fatal(err)

	case row == nil:
		//fmt.Print("-")
		// No entry.  Make one.
		err = fdb.insertOrReplace(f)
		if err != nil {
			log.Fatal(err)
		}

	default:
		//fmt.Print(".")
		// Got a value.  Compare it against calculated values.
		// If file was deliberately changed, replace the row with new cheksum
		err = nil
		switch {
		case !ignoreMTime && row["MOD_TIME"] != f.Info.ModTime().UnixNano():
			//fmt.Print("+")
			err = fdb.insertOrReplace(f)

		case row["CHKSUM"] != int64(f.Chksum):
			if ignoreMTime {
				f.Err = &myError{"Checksum has changed."}
			} else {
				f.Err = &myError{"Checksum has changed, even though mtime hasn't."}
			}
		}

		if err != nil {
			log.Fatal(err)
		}
	}

	return
}

func dbCompareChecker(in, out chan *compareJob, db *sqlite3.Conn) {
	defer func() { out <- nil }()

	fdb, err := newFileDB(db)
	if err != nil {
		fmt.Println("Cannot create prepared statements.")
		log.Fatal(err)
	}
	defer fdb.Close()

	for c := range in {
		if c == nil {
			return
		}

		if c.f1.Err == nil {
			CheckInDB(c.f1, fdb)
			if c.f1.Err != nil {
				c.description = "FAIL: "
			}
		}
		if c.f2.Err == nil {
			CheckInDB(c.f2, fdb)
			if c.f2.Err != nil {
				c.description = "FAIL: "
			}
		}

		out <- c
	}
}

func dbFileChecker(in chan *fileJob, out chan *compareJob, db *sqlite3.Conn) {
	defer func() { out <- nil }()

	fdb, err := newFileDB(db)
	if err != nil {
		fmt.Println("Cannot create prepared statements.")
		log.Fatal(err)
	}
	defer fdb.Close()

	for f := range in {
		if f == nil {
			return
		}

		c := new(compareJob)
		c.f1 = f
		if f.Err == nil {
			CheckInDB(f, fdb)
			if f.Err != nil {
				c.description = "FAIL: "
			}
		}

		out <- c
	}
}
