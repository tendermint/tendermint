# GoLevelDb performance tests

Simple tool to manipulate (insert & delete) records from a goLevelDb database.
The aim is to investigate how the database size fluctuates in time and how
it correlates with the number of records in the database.

Handles printing of the total size (in kb) of the database files and of
the number of records present in the db at various steps throughout a test.

Sample execution:


```txt
% go run one.go

			---- starting tests with GoLevelDB

			---- inserted 10000 records
			---- deleted 9000 records
			---- inserted 10000 records
			---- deleted 9000 records
			---- deleted 2000 records
	Steps:
	           name      size (kb)      records #
	{        initial               0               0}
	{         insert            1289           10000}
	{         delete            1562            1000}
	{         insert            2851           11000}
	{         delete            3123            2000}
	{         delete            3184               0}
```
