(window.webpackJsonp=window.webpackJsonp||[]).push([[181],{756:function(e,t,o){"use strict";o.r(t);var n=o(1),a=Object(n.a)({},(function(){var e=this,t=e.$createElement,o=e._self._c||t;return o("ContentSlotsDistributor",{attrs:{"slot-key":e.$parent.slotKey}},[o("h1",{attrs:{id:"tendermint-s-expected-behavior"}},[o("a",{staticClass:"header-anchor",attrs:{href:"#tendermint-s-expected-behavior"}},[e._v("#")]),e._v(" Tendermint's expected behavior")]),e._v(" "),o("h2",{attrs:{id:"valid-method-call-sequences"}},[o("a",{staticClass:"header-anchor",attrs:{href:"#valid-method-call-sequences"}},[e._v("#")]),e._v(" Valid method call sequences")]),e._v(" "),o("p",[e._v("This section describes what the Application can expect from Tendermint.")]),e._v(" "),o("p",[e._v("The Tendermint consensus algorithm is designed to protect safety under any network conditions, as long as\nless than 1/3 of validators' voting power is byzantine. Most of the time, though, the network will behave\nsynchronously, no process will fall behind, and there will be no byzantine process. The following describes\nwhat will happen during a block height "),o("em",[e._v("h")]),e._v(" in these frequent, benign conditions:")]),e._v(" "),o("ul",[o("li",[e._v("Tendermint will decide in round 0, for height "),o("em",[e._v("h")]),e._v(";")]),e._v(" "),o("li",[o("code",[e._v("PrepareProposal")]),e._v(" will be called exactly once at the proposer process of round 0, height "),o("em",[e._v("h")]),e._v(";")]),e._v(" "),o("li",[o("code",[e._v("ProcessProposal")]),e._v(" will be called exactly once at all processes, and\nwill return "),o("em",[e._v("accept")]),e._v(" in its "),o("code",[e._v("Response*")]),e._v(";")]),e._v(" "),o("li",[o("code",[e._v("ExtendVote")]),e._v(" will be called exactly once at all processes;")]),e._v(" "),o("li",[o("code",[e._v("VerifyVoteExtension")]),e._v(" will be called exactly "),o("em",[e._v("n-1")]),e._v(" times at each validator process, where "),o("em",[e._v("n")]),e._v(" is\nthe number of validators, and will always return "),o("em",[e._v("accept")]),e._v(" in its "),o("code",[e._v("Response*")]),e._v(";")]),e._v(" "),o("li",[o("code",[e._v("FinalizeBlock")]),e._v(" will be called exactly once at all processes, conveying the same prepared\nblock that all calls to "),o("code",[e._v("PrepareProposal")]),e._v(" and "),o("code",[e._v("ProcessProposal")]),e._v(" had previously reported for\nheight "),o("em",[e._v("h")]),e._v("; and")]),e._v(" "),o("li",[o("code",[e._v("Commit")]),e._v(" will finally be called exactly once at all processes at the end of height "),o("em",[e._v("h")]),e._v(".")])]),e._v(" "),o("p",[e._v("However, the Application logic must be ready to cope with any possible run of Tendermint for a given\nheight, including bad periods (byzantine proposers, network being asynchronous).\nIn these cases, the sequence of calls to ABCI++ methods may not be so straighforward, but\nthe Application should still be able to handle them, e.g., without crashing.\nThe purpose of this section is to define what these sequences look like an a precise way.")]),e._v(" "),o("p",[e._v("As mentioned in the "),o("RouterLink",{attrs:{to:"/spec/abci++/abci%2B%2B_basic_concepts.html"}},[e._v("Basic Concepts")]),e._v(" section, Tendermint\nacts as a client of ABCI++ and the Application acts as a server. Thus, it is up to Tendermint to\ndetermine when and in which order the different ABCI++ methods will be called. A well-written\nApplication design should consider "),o("em",[e._v("any")]),e._v(" of these possible sequences.")],1),e._v(" "),o("p",[e._v("The following grammar, written in case-sensitive Augmented Backus–Naur form (ABNF, specified\nin "),o("a",{attrs:{href:"https://datatracker.ietf.org/doc/html/rfc7405",target:"_blank",rel:"noopener noreferrer"}},[e._v("IETF rfc7405"),o("OutboundLink")],1),e._v("), specifies all possible\nsequences of calls to ABCI++, taken by a correct process, across all heights from the genesis block,\nincluding recovery runs, from the point of view of the Application.")]),e._v(" "),o("tm-code-block",{staticClass:"codeblock",attrs:{language:"abnf",base64:"c3RhcnQgICAgICAgICAgICAgICA9IGNsZWFuLXN0YXJ0IC8gcmVjb3ZlcnkKCmNsZWFuLXN0YXJ0ICAgICAgICAgPSBpbml0LWNoYWluIFtzdGF0ZS1zeW5jXSBjb25zZW5zdXMtZXhlYwpzdGF0ZS1zeW5jICAgICAgICAgID0gKnN0YXRlLXN5bmMtYXR0ZW1wdCBzdWNjZXNzLXN5bmMgaW5mbwpzdGF0ZS1zeW5jLWF0dGVtcHQgID0gb2ZmZXItc25hcHNob3QgKmFwcGx5LWNodW5rCnN1Y2Nlc3Mtc3luYyAgICAgICAgPSBvZmZlci1zbmFwc2hvdCAxKmFwcGx5LWNodW5rCgpyZWNvdmVyeSAgICAgICAgICAgID0gaW5mbyBjb25zZW5zdXMtZXhlYwoKY29uc2Vuc3VzLWV4ZWMgICAgICA9IChpbmYpY29uc2Vuc3VzLWhlaWdodApjb25zZW5zdXMtaGVpZ2h0ICAgID0gKmNvbnNlbnN1cy1yb3VuZCBkZWNpZGUgY29tbWl0CmNvbnNlbnN1cy1yb3VuZCAgICAgPSBwcm9wb3NlciAvIG5vbi1wcm9wb3NlcgoKcHJvcG9zZXIgICAgICAgICAgICA9ICpnb3Qtdm90ZSBwcmVwYXJlLXByb3Bvc2FsICpnb3Qtdm90ZSBwcm9jZXNzLXByb3Bvc2FsIFtleHRlbmRdCmV4dGVuZCAgICAgICAgICAgICAgPSAqZ290LXZvdGUgZXh0ZW5kLXZvdGUgKmdvdC12b3RlCm5vbi1wcm9wb3NlciAgICAgICAgPSAqZ290LXZvdGUgW3Byb2Nlc3MtcHJvcG9zYWxdIFtleHRlbmRdCgppbml0LWNoYWluICAgICAgICAgID0gJXMmcXVvdDsmbHQ7SW5pdENoYWluJmd0OyZxdW90OwpvZmZlci1zbmFwc2hvdCAgICAgID0gJXMmcXVvdDsmbHQ7T2ZmZXJTbmFwc2hvdCZndDsmcXVvdDsKYXBwbHktY2h1bmsgICAgICAgICA9ICVzJnF1b3Q7Jmx0O0FwcGx5U25hcHNob3RDaHVuayZndDsmcXVvdDsKaW5mbyAgICAgICAgICAgICAgICA9ICVzJnF1b3Q7Jmx0O0luZm8mZ3Q7JnF1b3Q7CnByZXBhcmUtcHJvcG9zYWwgICAgPSAlcyZxdW90OyZsdDtQcmVwYXJlUHJvcG9zYWwmZ3Q7JnF1b3Q7CnByb2Nlc3MtcHJvcG9zYWwgICAgPSAlcyZxdW90OyZsdDtQcm9jZXNzUHJvcG9zYWwmZ3Q7JnF1b3Q7CmV4dGVuZC12b3RlICAgICAgICAgPSAlcyZxdW90OyZsdDtFeHRlbmRWb3RlJmd0OyZxdW90Owpnb3Qtdm90ZSAgICAgICAgICAgID0gJXMmcXVvdDsmbHQ7VmVyaWZ5Vm90ZUV4dGVuc2lvbiZndDsmcXVvdDsKZGVjaWRlICAgICAgICAgICAgICA9ICVzJnF1b3Q7Jmx0O0ZpbmFsaXplQmxvY2smZ3Q7JnF1b3Q7CmNvbW1pdCAgICAgICAgICAgICAgPSAlcyZxdW90OyZsdDtDb21taXQmZ3Q7JnF1b3Q7Cg=="}}),e._v(" "),o("p",[e._v("We have kept some ABCI methods out of the grammar, in order to keep it as clear and concise as possible.\nA common reason for keeping all these methods out is that they all can be called at any point in a sequence defined\nby the grammar above. Other reasons depend on the method in question:")]),e._v(" "),o("ul",[o("li",[o("code",[e._v("Echo")]),e._v(" and "),o("code",[e._v("Flush")]),e._v(" are only used for debugging purposes. Further, their handling by the Application should be trivial.")]),e._v(" "),o("li",[o("code",[e._v("CheckTx")]),e._v(" is detached from the main method call sequence that drives block execution.")]),e._v(" "),o("li",[o("code",[e._v("Query")]),e._v(" provides read-only access to the current Application state, so handling it should also be independent from\nblock execution.")]),e._v(" "),o("li",[e._v("Similarly, "),o("code",[e._v("ListSnapshots")]),e._v(" and "),o("code",[e._v("LoadSnapshotChunk")]),e._v(" provide read-only access to the Application's previously created\nsnapshots (if any), and help populate the parameters of "),o("code",[e._v("OfferSnapshot")]),e._v(" and "),o("code",[e._v("ApplySnapshotChunk")]),e._v(" at a process performing\nstate-sync while bootstrapping. Unlike "),o("code",[e._v("ListSnapshots")]),e._v(" and "),o("code",[e._v("LoadSnapshotChunk")]),e._v(", both "),o("code",[e._v("OfferSnapshot")]),e._v("\nand "),o("code",[e._v("ApplySnapshotChunk")]),e._v(" "),o("em",[e._v("are")]),e._v(" included in the grammar.")])]),e._v(" "),o("p",[e._v("Finally, method "),o("code",[e._v("Info")]),e._v(" is a special case. The method's purpose is three-fold, it can be used")]),e._v(" "),o("ol",[o("li",[e._v("as part of handling an RPC call from an external client,")]),e._v(" "),o("li",[e._v("as a handshake between Tendermint and the Application upon recovery to check whether any blocks need\nto be replayed, and")]),e._v(" "),o("li",[e._v("at the end of "),o("em",[e._v("state-sync")]),e._v(" to verify that the correct state has been reached.")])]),e._v(" "),o("p",[e._v("We have left "),o("code",[e._v("Info")]),e._v("'s first purpose out of the grammar for the same reasons as all the others: it can happen\nat any time, and has nothing to do with the block execution sequence. The second and third purposes, on the other\nhand, are present in the grammar.")]),e._v(" "),o("p",[e._v("Let us now examine the grammar line by line, providing further details.")]),e._v(" "),o("ul",[o("li",[e._v("When a process starts, it may do so for the first time or after a crash (it is recovering).")])]),e._v(" "),o("blockquote",[o("tm-code-block",{staticClass:"codeblock",attrs:{language:"abnf",base64:"c3RhcnQgICAgICAgICAgICAgICA9IGNsZWFuLXN0YXJ0IC8gcmVjb3ZlcnkK"}})],1),e._v(" "),o("ul",[o("li",[e._v("If the process is starting from scratch, Tendermint first calls "),o("code",[e._v("InitChain")]),e._v(", then it may optionally\nstart a "),o("em",[e._v("state-sync")]),e._v(" mechanism to catch up with other processes. Finally, it enters normal\nconsensus execution.")])]),e._v(" "),o("blockquote",[o("tm-code-block",{staticClass:"codeblock",attrs:{language:"abnf",base64:"Y2xlYW4tc3RhcnQgICAgICAgICA9IGluaXQtY2hhaW4gW3N0YXRlLXN5bmNdIGNvbnNlbnN1cy1leGVjCg=="}})],1),e._v(" "),o("ul",[o("li",[e._v("In "),o("em",[e._v("state-sync")]),e._v(" mode, Tendermint makes one or more attempts at synchronizing the Application's state.\nAt the beginning of each attempt, it offers the Application a snapshot found at another process.\nIf the Application accepts the snapshot, a sequence of calls to "),o("code",[e._v("ApplySnapshotChunk")]),e._v(" method follow\nto provide the Application with all the snapshots needed, in order to reconstruct the state locally.\nA successful attempt must provide at least one chunk via "),o("code",[e._v("ApplySnapshotChunk")]),e._v(".\nAt the end of a successful attempt, Tendermint calls "),o("code",[e._v("Info")]),e._v(" to make sure the recontructed state's\n"),o("em",[e._v("AppHash")]),e._v(" matches the one in the block header at the corresponding height.")])]),e._v(" "),o("blockquote",[o("tm-code-block",{staticClass:"codeblock",attrs:{language:"abnf",base64:"c3RhdGUtc3luYyAgICAgICAgICA9ICpzdGF0ZS1zeW5jLWF0dGVtcHQgc3VjY2Vzcy1zeW5jIGluZm8Kc3RhdGUtc3luYy1hdHRlbXB0ICA9IG9mZmVyLXNuYXBzaG90ICphcHBseS1jaHVuawpzdWNjZXNzLXN5bmMgICAgICAgID0gb2ZmZXItc25hcHNob3QgMSphcHBseS1jaHVuawo="}})],1),e._v(" "),o("ul",[o("li",[e._v("In recovery mode, Tendermint first calls "),o("code",[e._v("Info")]),e._v(" to know from which height it needs to replay decisions\nto the Application. After this, Tendermint enters nomal consensus execution.")])]),e._v(" "),o("blockquote",[o("tm-code-block",{staticClass:"codeblock",attrs:{language:"abnf",base64:"cmVjb3ZlcnkgICAgICAgICAgICA9IGluZm8gY29uc2Vuc3VzLWV4ZWMK"}})],1),e._v(" "),o("ul",[o("li",[e._v("The non-terminal "),o("code",[e._v("consensus-exec")]),e._v(" is a key point in this grammar. It is an infinite sequence of\nconsensus heights. The grammar is thus an\n"),o("a",{attrs:{href:"https://dl.acm.org/doi/10.5555/2361476.2361481",target:"_blank",rel:"noopener noreferrer"}},[e._v("omega-grammar"),o("OutboundLink")],1),e._v(", since it produces infinite\nsequences of terminals (i.e., the API calls).")])]),e._v(" "),o("blockquote",[o("tm-code-block",{staticClass:"codeblock",attrs:{language:"abnf",base64:"Y29uc2Vuc3VzLWV4ZWMgICAgICA9IChpbmYpY29uc2Vuc3VzLWhlaWdodAo="}})],1),e._v(" "),o("ul",[o("li",[e._v("A consensus height consists of zero or more rounds before deciding and executing via a call to\n"),o("code",[e._v("FinalizeBlock")]),e._v(", followed by a call to "),o("code",[e._v("Commit")]),e._v(". In each round, the sequence of method calls\ndepends on whether the local process is the proposer or not. Note that, if a height contains zero\nrounds, this means the process is replaying an already decided value (catch-up mode).")])]),e._v(" "),o("blockquote",[o("tm-code-block",{staticClass:"codeblock",attrs:{language:"abnf",base64:"Y29uc2Vuc3VzLWhlaWdodCAgICA9ICpjb25zZW5zdXMtcm91bmQgZGVjaWRlIGNvbW1pdApjb25zZW5zdXMtcm91bmQgICAgID0gcHJvcG9zZXIgLyBub24tcHJvcG9zZXIK"}})],1),e._v(" "),o("ul",[o("li",[e._v("For every round, if the local process is the proposer of the current round, Tendermint starts by\ncalling "),o("code",[e._v("PrepareProposal")]),e._v(", followed by "),o("code",[e._v("ProcessProposal")]),e._v(". Then, optionally, the Application is\nasked to extend its vote for that round. Calls to "),o("code",[e._v("VerifyVoteExtension")]),e._v(" can come at any time: the\nlocal process may be slightly late in the current round, or votes may come from a future round\nof this height.")])]),e._v(" "),o("blockquote",[o("tm-code-block",{staticClass:"codeblock",attrs:{language:"abnf",base64:"cHJvcG9zZXIgICAgICAgICAgICA9ICpnb3Qtdm90ZSBwcmVwYXJlLXByb3Bvc2FsICpnb3Qtdm90ZSBwcm9jZXNzLXByb3Bvc2FsIFtleHRlbmRdCmV4dGVuZCAgICAgICAgICAgICAgPSAqZ290LXZvdGUgZXh0ZW5kLXZvdGUgKmdvdC12b3RlCg=="}})],1),e._v(" "),o("ul",[o("li",[e._v("Also for every round, if the local process is "),o("em",[e._v("not")]),e._v(" the proposer of the current round, Tendermint\nwill call "),o("code",[e._v("ProcessProposal")]),e._v(" at most once. At most one call to "),o("code",[e._v("ExtendVote")]),e._v(" may occur only after\n"),o("code",[e._v("ProcessProposal")]),e._v(" is called. A number of calls to "),o("code",[e._v("VerifyVoteExtension")]),e._v(" can occur in any order\nwith respect to "),o("code",[e._v("ProcessProposal")]),e._v(" and "),o("code",[e._v("ExtendVote")]),e._v(" throughout the round. The reasons are the same\nas above, namely, the process running slightly late in the current round, or votes from future\nrounds of this height received.")])]),e._v(" "),o("blockquote",[o("tm-code-block",{staticClass:"codeblock",attrs:{language:"abnf",base64:"bm9uLXByb3Bvc2VyICAgICAgICA9ICpnb3Qtdm90ZSBbcHJvY2Vzcy1wcm9wb3NhbF0gW2V4dGVuZF0K"}})],1),e._v(" "),o("ul",[o("li",[e._v("Finally, the grammar describes all its terminal symbols, which denote the different ABCI++ method calls that\nmay appear in a sequence.")])]),e._v(" "),o("blockquote",[o("tm-code-block",{staticClass:"codeblock",attrs:{language:"abnf",base64:"aW5pdC1jaGFpbiAgICAgICAgICA9ICVzJnF1b3Q7Jmx0O0luaXRDaGFpbiZndDsmcXVvdDsKb2ZmZXItc25hcHNob3QgICAgICA9ICVzJnF1b3Q7Jmx0O09mZmVyU25hcHNob3QmZ3Q7JnF1b3Q7CmFwcGx5LWNodW5rICAgICAgICAgPSAlcyZxdW90OyZsdDtBcHBseVNuYXBzaG90Q2h1bmsmZ3Q7JnF1b3Q7CmluZm8gICAgICAgICAgICAgICAgPSAlcyZxdW90OyZsdDtJbmZvJmd0OyZxdW90OwpwcmVwYXJlLXByb3Bvc2FsICAgID0gJXMmcXVvdDsmbHQ7UHJlcGFyZVByb3Bvc2FsJmd0OyZxdW90Owpwcm9jZXNzLXByb3Bvc2FsICAgID0gJXMmcXVvdDsmbHQ7UHJvY2Vzc1Byb3Bvc2FsJmd0OyZxdW90OwpleHRlbmQtdm90ZSAgICAgICAgID0gJXMmcXVvdDsmbHQ7RXh0ZW5kVm90ZSZndDsmcXVvdDsKZ290LXZvdGUgICAgICAgICAgICA9ICVzJnF1b3Q7Jmx0O1ZlcmlmeVZvdGVFeHRlbnNpb24mZ3Q7JnF1b3Q7CmRlY2lkZSAgICAgICAgICAgICAgPSAlcyZxdW90OyZsdDtGaW5hbGl6ZUJsb2NrJmd0OyZxdW90Owpjb21taXQgICAgICAgICAgICAgID0gJXMmcXVvdDsmbHQ7Q29tbWl0Jmd0OyZxdW90Owo="}})],1),e._v(" "),o("h2",{attrs:{id:"adapting-existing-applications-that-use-abci"}},[o("a",{staticClass:"header-anchor",attrs:{href:"#adapting-existing-applications-that-use-abci"}},[e._v("#")]),e._v(" Adapting existing Applications that use ABCI")]),e._v(" "),o("p",[e._v("In some cases, an existing Application using the legacy ABCI may need to be adapted to work with ABCI++\nwith as minimal changes as possible. In this case, of course, ABCI++ will not provide any advange with respect\nto the existing implementation, but will keep the same guarantees already provided by ABCI.\nHere is how ABCI++ methods should be implemented.")]),e._v(" "),o("p",[e._v("First of all, all the methods that did not change from ABCI to ABCI++, namely "),o("code",[e._v("Echo")]),e._v(", "),o("code",[e._v("Flush")]),e._v(", "),o("code",[e._v("Info")]),e._v(", "),o("code",[e._v("InitChain")]),e._v(",\n"),o("code",[e._v("Query")]),e._v(", "),o("code",[e._v("CheckTx")]),e._v(", "),o("code",[e._v("ListSnapshots")]),e._v(", "),o("code",[e._v("LoadSnapshotChunk")]),e._v(", "),o("code",[e._v("OfferSnapshot")]),e._v(", and "),o("code",[e._v("ApplySnapshotChunk")]),e._v(", do not need\nto undergo any changes in their implementation.")]),e._v(" "),o("p",[e._v("As for the new methods:")]),e._v(" "),o("ul",[o("li",[o("code",[e._v("PrepareProposal")]),e._v(" must create a list of "),o("RouterLink",{attrs:{to:"/spec/abci++/abci++_methods.html#txrecord"}},[e._v("TxRecord")]),e._v(" each containing\na transaction passed in "),o("code",[e._v("RequestPrepareProposal.txs")]),e._v(", in the same other. The field "),o("code",[e._v("action")]),e._v(" must\nbe set to "),o("code",[e._v("UNMODIFIED")]),e._v(" for all "),o("RouterLink",{attrs:{to:"/spec/abci++/abci++_methods.html#txrecord"}},[e._v("TxRecord")]),e._v(" elements in the list.\nThe Application must check whether the size of all transactions exceeds the byte limit\n("),o("code",[e._v("RequestPrepareProposal.max_tx_bytes")]),e._v("). If so, the Application must remove transactions at the\nend of the list until the total byte size is at or below the limit.")],1),e._v(" "),o("li",[o("code",[e._v("ProcessProposal")]),e._v(" must set "),o("code",[e._v("ResponseProcessProposal.accept")]),e._v(" to "),o("em",[e._v("true")]),e._v(" and return.")]),e._v(" "),o("li",[o("code",[e._v("ExtendVote")]),e._v(" is to set "),o("code",[e._v("ResponseExtendVote.extension")]),e._v(" to an empty byte array and return.")]),e._v(" "),o("li",[o("code",[e._v("VerifyVoteExtension")]),e._v(" must set "),o("code",[e._v("ResponseVerifyVoteExtension.accept")]),e._v(" to "),o("em",[e._v("true")]),e._v(" if the extension is\nan empty byte array and "),o("em",[e._v("false")]),e._v(" otherwise, then return.")]),e._v(" "),o("li",[o("code",[e._v("FinalizeBlock")]),e._v(" is to coalesce the implementation of methods "),o("code",[e._v("BeginBlock")]),e._v(", "),o("code",[e._v("DeliverTx")]),e._v(", and\n"),o("code",[e._v("EndBlock")]),e._v(". Legacy applications looking to reuse old code that implemented "),o("code",[e._v("DeliverTx")]),e._v(" should\nwrap the legacy "),o("code",[e._v("DeliverTx")]),e._v(" logic in a loop that executes one transaction iteration per\ntransaction in "),o("code",[e._v("RequestFinalizeBlock.tx")]),e._v(".")])]),e._v(" "),o("p",[e._v("Finally, "),o("code",[e._v("Commit")]),e._v(", which is kept in ABCI++, no longer returns the "),o("code",[e._v("AppHash")]),e._v(". It is now up to\n"),o("code",[e._v("FinalizeBlock")]),e._v(" to do so. Thus, a slight refactoring of the old "),o("code",[e._v("Commit")]),e._v(" implementation will be\nneeded to move the return of "),o("code",[e._v("AppHash")]),e._v(" to "),o("code",[e._v("FinalizeBlock")]),e._v(".")])],1)}),[],!1,null,null,null);t.default=a.exports}}]);