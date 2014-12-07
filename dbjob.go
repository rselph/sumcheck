package main

import (
	"io"
	"log"

	"code.google.com/p/go-sqlite/go1/sqlite3"
)

const batchSize int = 100

type FileDB struct {
	db *sqlite3.Conn

	insert     *sqlite3.Stmt
	batchCount int

	get     *sqlite3.Stmt
	getArgs sqlite3.NamedArgs
	//row    sqlite3.RowMap
}

func NewFileDB(path string) (fdb *FileDB, err error) {
	fdb = &FileDB{}
	fdb.db, err = sqlite3.Open(path)
	if err != nil {
		return nil, err
	}

	err = fdb.db.Exec("CREATE TABLE IF NOT EXISTS FILES (PATH varchar not null, SIZE integer not null, MOD_TIME integer not null, MODE integer not null, CHKSUM integer not null, primary key(PATH));")
	if err != nil {
		fdb.Close()
		return nil, err
	}

	//err = fdb.db.Exec("CREATE INDEX IF NOT EXISTS BYSUM on FILES (CHKSUM, SIZE);")
	//if err != nil {
	//	fdb.Close()
	//	return nil, err
	//}

	fdb.insert, err = fdb.db.Prepare("insert or replace into FILES (path, size, mod_time, mode, chksum) values ($path, $size, $mod_time, $mode, $chksum);")
	if err != nil {
		fdb.Close()
		return nil, err
	}

	fdb.get, err = fdb.db.Prepare("select size, mod_time, mode, chksum from FILES where path = $path;")
	if err != nil {
		fdb.Close()
		return nil, err
	}

	fdb.getArgs = make(sqlite3.NamedArgs)

	err = fdb.db.Begin()
	if err != nil {
		fdb.Close()
		return nil, err
	}

	return
}

func (fdb *FileDB) Close() {
	if fdb.db != nil {
		fdb.db.Commit()

		if fdb.insert != nil {
			fdb.insert.Close()
		}
		if fdb.get != nil {
			fdb.get.Close()
		}
		fdb.db.Close()
	}
}

func (fdb *FileDB) insertOrReplace(f *fileJob) {
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
		err = fdb.db.Commit()
		if err != nil {
			log.Fatal(err)
		}
		fdb.db.Begin()
		if err != nil {
			log.Fatal(err)
		}
		fdb.batchCount = 0
	}

	f.Err = NewError(code_NEW_SUM, f, "updating checksum")
	return
}

func (fdb *FileDB) getForPath(f *fileJob) (row sqlite3.RowMap, err error) {
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

func (fdb *FileDB) CheckInDB(f *fileJob) {
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

func dbChecker(in chan *fileJob, out chan *fileJob, fdb *FileDB) {
	defer func() { out <- nil }()

	for f := range in {
		if f == nil {
			return
		}

		if f.Err == nil {
			fdb.CheckInDB(f)
		}

		out <- f
	}
}
