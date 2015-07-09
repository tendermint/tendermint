1. Fork github.com/tendermint/tendermint.
2. Run "make", it should install the daemon, which we named "tendermint".
3. Run "tendermint gen_account".  Save the address, pub_key bytes, and priv_key bytes.
   This is your developer key for controlling the cloud nodes.
4. Also run "tendermint gen_validator" 5 times, once for each cloud node.  Save the output.
5. Create a directory ~/.debora/ and copy cmd/debora/default.cfg into ~/.debora/default.cfg
   Copy the priv_key bytes from step 4 into ~/.debora/default.cfg where it says so.
   Change the list of hosts in ~/.debora/default.cfg with your own set of 5 cloud nodes.
6. Replace cmd/barak/seed's pubkey with the pub_key bytes from step 3.
7. Update config/tendermint/config.go's genesis with validator pubkeys from step 4.
   Give each of your nodes the same amount of voting power.
   Set up the accounts however you want.
8. On each cloud node, follow the instructions here: https://github.com/tendermint/tendermint/tree/master/INSTALL
   Create tmuser, install go, and also install 'barak'.
   Then, run `barak -config="cmd/barak/seed"`.
   You don't need to start the node at this time.
9. Now you can run "debora list" on your development machine and post commands to each cloud node.
10. Run scripts/unsafe_upgrade_barak.sh to test that barak is running.
    The old barak you started on step 8 should now have quit.
    A new instance of barak should be running.  Check with `ps -ef | grep "barak"`
11. Run scripts/unsafe_restart_net.sh start your new testnet.
