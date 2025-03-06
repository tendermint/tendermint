(window.webpackJsonp=window.webpackJsonp||[]).push([[210],{786:function(e,t,o){"use strict";o.r(t);var n=o(1),i=Object(n.a)({},(function(){var e=this,t=e.$createElement,o=e._self._c||t;return o("ContentSlotsDistributor",{attrs:{"slot-key":e.$parent.slotKey}},[o("h1",{attrs:{id:"ivy-proofs"}},[o("a",{staticClass:"header-anchor",attrs:{href:"#ivy-proofs"}},[e._v("#")]),e._v(" Ivy Proofs")]),e._v(" "),o("tm-code-block",{staticClass:"codeblock",attrs:{language:"copyright",base64:"Q29weXJpZ2h0IChjKSAyMDIwIEdhbG9pcywgSW5jLgpTUERYLUxpY2Vuc2UtSWRlbnRpZmllcjogQXBhY2hlLTIuMAo="}}),e._v(" "),o("h2",{attrs:{id:"contents"}},[o("a",{staticClass:"header-anchor",attrs:{href:"#contents"}},[e._v("#")]),e._v(" Contents")]),e._v(" "),o("p",[e._v("This folder contains:")]),e._v(" "),o("ul",[o("li",[o("code",[e._v("tendermint.ivy")]),e._v(", a specification of Tendermint algorithm as described in "),o("em",[e._v("The latest gossip on BFT consensus")]),e._v(" by E. Buchman, J. Kwon, Z. Milosevic.")]),e._v(" "),o("li",[o("code",[e._v("abstract_tendermint.ivy")]),e._v(", a more abstract specification of Tendermint that is more verification-friendly.")]),e._v(" "),o("li",[o("code",[e._v("classic_safety.ivy")]),e._v(", a proof that Tendermint satisfies the classic safety property of BFT consensus: if every two quorums have a well-behaved node in common, then no two well-behaved nodes ever disagree.")]),e._v(" "),o("li",[o("code",[e._v("accountable_safety_1.ivy")]),e._v(", a proof that, assuming every quorum contains at least one well-behaved node, if two well-behaved nodes disagree, then there is evidence demonstrating at least f+1 nodes misbehaved.")]),e._v(" "),o("li",[o("code",[e._v("accountable_safety_2.ivy")]),e._v(", a proof that, regardless of any assumption about quorums, well-behaved nodes cannot be framed by malicious nodes. In other words, malicious nodes can never construct evidence that incriminates a well-behaved node.")]),e._v(" "),o("li",[o("code",[e._v("network_shim.ivy")]),e._v(", the network model and a convenience "),o("code",[e._v("shim")]),e._v(" object to interface with the Tendermint specification.")]),e._v(" "),o("li",[o("code",[e._v("domain_model.ivy")]),e._v(", a specification of the domain model underlying the Tendermint specification, i.e. rounds, value, quorums, etc.")])]),e._v(" "),o("p",[e._v("All specifications and proofs are written in "),o("a",{attrs:{href:"https://github.com/kenmcmil/ivy",target:"_blank",rel:"noopener noreferrer"}},[e._v("Ivy"),o("OutboundLink")],1),e._v(".")]),e._v(" "),o("p",[e._v("The license above applies to all files in this folder.")]),e._v(" "),o("h2",{attrs:{id:"building-and-running"}},[o("a",{staticClass:"header-anchor",attrs:{href:"#building-and-running"}},[e._v("#")]),e._v(" Building and running")]),e._v(" "),o("p",[e._v("The easiest way to check the proofs is to use "),o("a",{attrs:{href:"https://www.docker.com/",target:"_blank",rel:"noopener noreferrer"}},[e._v("Docker"),o("OutboundLink")],1),e._v(".")]),e._v(" "),o("ol",[o("li",[e._v("Install "),o("a",{attrs:{href:"https://docs.docker.com/get-docker/",target:"_blank",rel:"noopener noreferrer"}},[e._v("Docker"),o("OutboundLink")],1),e._v(" and "),o("a",{attrs:{href:"https://docs.docker.com/compose/install/",target:"_blank",rel:"noopener noreferrer"}},[e._v("Docker Compose"),o("OutboundLink")],1),e._v(".")]),e._v(" "),o("li",[e._v("Build a Docker image: "),o("code",[e._v("docker-compose build")])]),e._v(" "),o("li",[e._v("Run the proofs inside the Docker container: "),o("code",[e._v("docker-compose run tendermint-proof")]),e._v(". This will check all the proofs with the "),o("code",[e._v("ivy_check")]),e._v("\ncommand and write the output of "),o("code",[e._v("ivy_check")]),e._v(" to a subdirectory of `./output/'")])])],1)}),[],!1,null,null,null);t.default=i.exports}}]);