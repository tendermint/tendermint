package rpc

//
// // TestABCIQuery tests ABCIQuery requests and verifies proofs. HAPPY PATH 😀
// func TestABCIQuery(t *testing.T) {
//	tree, err := iavl.NewMutableTree(dbm.NewMemDB(), 100)
//	require.NoError(t, err)
//
//	var (
//		key   = []byte("foo")
//		value = []byte("bar")
//	)
//	tree.Set(key, value)
//
//	commitmentProof, err := tree.GetMembershipProof(key)
//	require.NoError(t, err)
//
//	op := &testOp{
//		Spec:  ics23.IavlSpec,
//		Key:   key,
//		Proof: commitmentProof,
//	}
//
//	next := &rpcmock.Client{}
//	next.On(
//		"ABCIQueryWithOptions",
//		context.Background(),
//		mock.AnythingOfType("string"),
//		bytes.HexBytes(key),
//		mock.AnythingOfType("client.ABCIQueryOptions"),
//	).Return(&ctypes.ResultABCIQuery{
//		Response: abci.ResponseQuery{
//			Code:   0,
//			Key:    key,
//			Value:  value,
//			Height: 1,
//			ProofOps: &tmcrypto.ProofOps{
//				Ops: []tmcrypto.ProofOp{op.ProofOp()},
//			},
//		},
//	}, nil)
//
//	lc := &lcmock.LightClient{}
//	appHash, _ := hex.DecodeString("5EFD44055350B5CC34DBD26085347A9DBBE44EA192B9286A9FC107F40EA1FAC5")
//	lc.On("VerifyLightBlockAtHeight", context.Background(), int64(2), mock.AnythingOfType("time.Time")).Return(
//		&types.LightBlock{
//			SignedHeader: &types.SignedHeader{
//				Header: &types.Header{AppHash: appHash},
//			},
//		},
//		nil,
//	)
//
//	c := NewClient(next, lc,
//		KeyPathFn(func(_ string, key []byte) (merkle.KeyPath, error) {
//			kp := merkle.KeyPath{}
//			kp = kp.AppendKey(key, merkle.KeyEncodingURL)
//			return kp, nil
//		}))
//	c.RegisterOpDecoder("ics23:iavl", testOpDecoder)
//	res, err := c.ABCIQuery(context.Background(), "/store/accounts/key", key)
//	require.NoError(t, err)
//
//	assert.NotNil(t, res)
// }
//
// type testOp struct {
//	Spec  *ics23.ProofSpec
//	Key   []byte
//	Proof *ics23.CommitmentProof
// }
//
// var _ merkle.ProofOperator = testOp{}
//
// func (op testOp) GetKey() []byte {
//	return op.Key
// }
//
// func (op testOp) ProofOp() tmcrypto.ProofOp {
//	bz, err := op.Proof.Marshal()
//	if err != nil {
//		panic(err.Error())
//	}
//	return tmcrypto.ProofOp{
//		Type: "ics23:iavl",
//		Key:  op.Key,
//		Data: bz,
//	}
// }
//
// func (op testOp) Run(args [][]byte) ([][]byte, error) {
//	// calculate root from proof
//	root, err := op.Proof.Calculate()
//	if err != nil {
//		return nil, fmt.Errorf("could not calculate root for proof: %v", err)
//	}
//	// Only support an existence proof or nonexistence proof (batch proofs currently unsupported)
//	switch len(args) {
//	case 0:
//		// Args are nil, so we verify the absence of the key.
//		absent := ics23.VerifyNonMembership(op.Spec, root, op.Proof, op.Key)
//		if !absent {
//			return nil, fmt.Errorf("proof did not verify absence of key: %s", string(op.Key))
//		}
//	case 1:
//		// Args is length 1, verify existence of key with value args[0]
//		if !ics23.VerifyMembership(op.Spec, root, op.Proof, op.Key, args[0]) {
//			return nil, fmt.Errorf("proof did not verify existence of key %s with given value %x", op.Key, args[0])
//		}
//	default:
//		return nil, fmt.Errorf("args must be length 0 or 1, got: %d", len(args))
//	}
//
//	return [][]byte{root}, nil
// }
//
// func testOpDecoder(pop tmcrypto.ProofOp) (merkle.ProofOperator, error) {
//	proof := &ics23.CommitmentProof{}
//	err := proof.Unmarshal(pop.Data)
//	if err != nil {
//		return nil, err
//	}
//
//	op := testOp{
//		Key:   pop.Key,
//		Spec:  ics23.IavlSpec,
//		Proof: proof,
//	}
//	return op, nil
// }
