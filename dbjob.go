// dbjob.go
package main

import (
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

func newFileDB(db *sqlite3.Conn) (fdb *fileDB) {
	var err error
	fdb = &fileDB{}

	fdb.insert, err = db.Prepare("insert or replace into FILES (path, size, mod_time, mode, chksum) values ($path, $size, $mod_time, $mode, $chksum);")
	if err != nil {
		fdb.Close()
		log.Fatal(err)
	}

	fdb.get, err = db.Prepare("select size, mod_time, mode, chksum from FILES where path = $path;")
	if err != nil {
		fdb.Close()
		log.Fatal(err)
	}

	fdb.begin, err = db.Prepare("BEGIN TRANSACTION;")
	if err != nil {
		fdb.Close()
		log.Fatal(err)
	}

	fdb.commit, err = db.Prepare("COMMIT TRANSACTION;")
	if err != nil {
		fdb.Close()
		log.Fatal(err)
	}

	fdb.getArgs = make(sqlite3.NamedArgs)

	err = fdb.begin.Exec()
	if err != nil {
		fdb.Close()
		log.Fatal(err)
	}

	return
}

func (fdb *fileDB) insertOrReplace(f *fileJob) {
	args := make(sqlite3.NamedArgs)
	args["$path"] = f.Fpath
	args["$size"] = f.Info.Size()
	args["$mod_time"] = f.Info.ModTime().UnixNano()
	args["$mode"] = int64(f.Info.Mode())
	args["$chksum"] = int64(f.Chksum)

	defer fdb.insert.Reset()
	err := fdb.insert.Exec(args)
	if err != nil {
		log.Fatal(err)
	}

	if fdb.batchCount += 1; fdb.batchCount >= batchSize {
		err = fdb.commit.Exec()
		if err != nil {
			log.Fatal(err)
		}
		fdb.begin.Exec()
		if err != nil {
			log.Fatal(err)
		}
		fdb.batchCount = 0
	}

	f.Err = NewError(code_NEW_SUM, f, "updating checksum")
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
		// No entry.  Make one.
		fdb.insertOrReplace(f)

	default:
		// Got a value.  Compare it against calculated values.
		// If file was deliberately changed, replace the row with new cheksum
		switch {
		case !ignoreMTime && row["MOD_TIME"] != f.Info.ModTime().UnixNano():
			fdb.insertOrReplace(f)

		case row["CHKSUM"] != int64(f.Chksum):
			f.Err = NewError(code_BAD_SUM, f, "checksum did not match recorded value")
		}
	}

	return
}

func dbChecker(in chan *fileJob, out chan *fileJob, db *sqlite3.Conn) {
	defer func() { out <- nil }()

	fdb := newFileDB(db)
	defer fdb.Close()

	for f := range in {
		if f == nil {
			return
		}

		if f.Err == nil {
			CheckInDB(f, fdb)
		}

		out <- f
	}
}
