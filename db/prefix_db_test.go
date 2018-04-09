package db

import "testing"

func TestIteratePrefix(t *testing.T) {
	db := NewMemDB()
	// Under "key" prefix
	db.Set(bz("key"), bz("value"))
	db.Set(bz("key1"), bz("value1"))
	db.Set(bz("key2"), bz("value2"))
	db.Set(bz("key3"), bz("value3"))
	db.Set(bz("something"), bz("else"))
	db.Set(bz(""), bz(""))
	db.Set(bz("k"), bz("val"))
	db.Set(bz("ke"), bz("valu"))
	db.Set(bz("kee"), bz("valuu"))
	xitr := db.Iterator(nil, nil)
	xitr.Key()

	pdb := NewPrefixDB(db, bz("key"))
	checkValue(t, pdb, bz("key"), nil)
	checkValue(t, pdb, bz(""), bz("value"))
	checkValue(t, pdb, bz("key1"), nil)
	checkValue(t, pdb, bz("1"), bz("value1"))
	checkValue(t, pdb, bz("key2"), nil)
	checkValue(t, pdb, bz("2"), bz("value2"))
	checkValue(t, pdb, bz("key3"), nil)
	checkValue(t, pdb, bz("3"), bz("value3"))
	checkValue(t, pdb, bz("something"), nil)
	checkValue(t, pdb, bz("k"), nil)
	checkValue(t, pdb, bz("ke"), nil)
	checkValue(t, pdb, bz("kee"), nil)

	itr := pdb.Iterator(nil, nil)
	itr.Key()
	checkItem(t, itr, bz(""), bz("value"))
	checkNext(t, itr, true)
	checkItem(t, itr, bz("1"), bz("value1"))
	checkNext(t, itr, true)
	checkItem(t, itr, bz("2"), bz("value2"))
	checkNext(t, itr, true)
	checkItem(t, itr, bz("3"), bz("value3"))
	itr.Close()
}
