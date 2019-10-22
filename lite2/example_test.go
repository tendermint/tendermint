package lite

//func TestExample_Client(t *testing.T) {
//	const (
//		chainID = "my-awesome-chain"
//	)
//	dbDir, err := ioutil.TempDir("", "lite-client-example")
//	if err != nil {
//		t.Fatal(err)
//	}
//	defer os.RemoveAll(dbDir)

//	// TODO: fetch the "trusted" header from a node
//	header := (*types.SignedHeader)(nil)

//	/////////////////////////////////////////////////////////////////////////////

//	db, err := dbm.NewGoLevelDB("lite-client-db", dbDir)
//	if err != nil {
//		// return err
//		t.Fatal(err)
//	}
//	c, err := NewClient(
//		chainID,
//		TrustOptions{
//			Period: 504 * time.Hour, // 21 days
//			Height: 100,
//			Hash:   header.Hash(),
//		},
//		httpp.New(chainID, "tcp://localhost:26657"),
//		dbs.New(db, chainID),
//	)

//	err = c.VerifyHeaderAtHeight(101, time.Now())
//	if err != nil {
//		fmt.Println("retry?")
//	}

//	h, err := c.TrustedHeader(101)
//	if err != nil {
//		fmt.Println("retry?")
//	}
//	fmt.Println("got header", h)
//	// verify some data
//}

//func TestExample_AutoClient(t *testing.T) {
//	const (
//		chainID = "my-awesome-chain"
//	)
//	dbDir, err := ioutil.TempDir("", "lite-client-example")
//	if err != nil {
//		t.Fatal(err)
//	}
//	defer os.RemoveAll(dbDir)

//	// TODO: fetch the "trusted" header from a node
//	header := (*types.SignedHeader)(nil)

//	/////////////////////////////////////////////////////////////////////////////

//	db, err := dbm.NewGoLevelDB("lite-client-db", dbDir)
//	if err != nil {
//		// return err
//		t.Fatal(err)
//	}

//	base, err := NewClient(
//		chainID,
//		TrustOptions{
//			Period: 504 * time.Hour, // 21 days
//			Height: 100,
//			Hash:   header.Hash(),
//		},
//		httpp.New(chainID, "tcp://localhost:26657"),
//		dbs.New(db, chainID),
//	)

//	c := NewAutoClient(base, 1*time.Second)
//	defer c.Stop()

//	select {
//	case h := <-c.TrustedHeaders():
//		fmt.Println("got header", h)
//		// verify some data
//	case err := <-c.Err():
//		switch errors.Cause(err).(type) {
//		case ErrOldHeaderExpired:
//			// reobtain trust height and hash
//		default:
//			// try with another full node
//			fmt.Println("got error", err)
//		}
//	}
//}
