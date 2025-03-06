(window.webpackJsonp=window.webpackJsonp||[]).push([[157],{733:function(e,t,n){"use strict";n.r(t);var o=n(1),s=Object(o.a)({},(function(){var e=this,t=e.$createElement,n=e._self._c||t;return n("ContentSlotsDistributor",{attrs:{"slot-key":e.$parent.slotKey}},[n("h1",{attrs:{id:"rfc-006-event-subscription"}},[n("a",{staticClass:"header-anchor",attrs:{href:"#rfc-006-event-subscription"}},[e._v("#")]),e._v(" RFC 006: Event Subscription")]),e._v(" "),n("h2",{attrs:{id:"changelog"}},[n("a",{staticClass:"header-anchor",attrs:{href:"#changelog"}},[e._v("#")]),e._v(" Changelog")]),e._v(" "),n("ul",[n("li",[e._v("30-Oct-2021: Initial draft (@creachadair)")])]),e._v(" "),n("h2",{attrs:{id:"abstract"}},[n("a",{staticClass:"header-anchor",attrs:{href:"#abstract"}},[e._v("#")]),e._v(" Abstract")]),e._v(" "),n("p",[e._v("The Tendermint consensus node allows clients to subscribe to its event stream\nvia methods on its RPC service.  The ability to view the event stream is\nvaluable for clients, but the current implementation has some deficiencies that\nmake it difficult for some clients to use effectively. This RFC documents these\nissues and discusses possible approaches to solving them.")]),e._v(" "),n("h2",{attrs:{id:"background"}},[n("a",{staticClass:"header-anchor",attrs:{href:"#background"}},[e._v("#")]),e._v(" Background")]),e._v(" "),n("p",[e._v("A running Tendermint consensus node exports a "),n("a",{attrs:{href:"https://docs.tendermint.com/master/rpc/",target:"_blank",rel:"noopener noreferrer"}},[e._v("JSON-RPC service"),n("OutboundLink")],1),e._v("\nthat provides a "),n("a",{attrs:{href:"https://github.com/tendermint/tendermint/blob/master/internal/rpc/core/routes.go#L12",target:"_blank",rel:"noopener noreferrer"}},[e._v("large set of methods"),n("OutboundLink")],1),e._v(" for inspecting and\ninteracting with the node.  One important cluster of these methods are the\n"),n("code",[e._v("subscribe")]),e._v(", "),n("code",[e._v("unsubscribe")]),e._v(", and "),n("code",[e._v("unsubscribe_all")]),e._v(" methods, which permit clients\nto subscribe to a filtered stream of the "),n("a",{attrs:{href:"./rfc-005-event-system.rst"}},[e._v("events generated by the node")]),e._v("\nas it runs.")]),e._v(" "),n("p",[e._v('Unlike the other methods of the service, the methods in the "event\nsubscription" cluster are not accessible via '),n("RouterLink",{attrs:{to:"/rfc/rfc-002-ipc-ecosystem.html#rpc-transport"}},[e._v("ordinary HTTP GET or POST\nrequests")]),e._v(", but require upgrading the HTTP connection to a\n"),n("a",{attrs:{href:"https://datatracker.ietf.org/doc/html/rfc6455",target:"_blank",rel:"noopener noreferrer"}},[e._v("websocket"),n("OutboundLink")],1),e._v(".  This is necessary because the "),n("code",[e._v("subscribe")]),e._v(" request needs a\npersistent channel to deliver results back to the client, and an ordinary HTTP\nconnection does not reliably persist across multiple requests.  Since these\nmethods do not work properly without a persistent channel, they are "),n("em",[e._v("only")]),e._v("\nexported via a websocket connection, and are not routed for plain HTTP.")],1),e._v(" "),n("h2",{attrs:{id:"discussion"}},[n("a",{staticClass:"header-anchor",attrs:{href:"#discussion"}},[e._v("#")]),e._v(" Discussion")]),e._v(" "),n("p",[e._v("There are some operational problems with the current implementation of event\nsubscription in the RPC service:")]),e._v(" "),n("ul",[n("li",[n("p",[n("strong",[e._v("Event delivery is not valid JSON-RPC.")]),e._v(" When a client issues a "),n("code",[e._v("subscribe")]),e._v("\nrequest, the server replies (correctly) with an initial empty acknowledgement\n("),n("code",[e._v("{}")]),e._v('). After that, each matching event is delivered "unsolicited" (without\nanother request from the client), as a separate '),n("a",{attrs:{href:"https://www.jsonrpc.org/specification#response_object",target:"_blank",rel:"noopener noreferrer"}},[e._v("response object"),n("OutboundLink")],1),e._v("\nwith the same ID as the initial request.")]),e._v(" "),n("p",[e._v("This matters because it means a standard JSON-RPC client library can't\ninteract correctly with the event subscription mechanism.")]),e._v(" "),n("p",[e._v("Even for clients that can handle unsolicited values pushed by the server,\nthese responses are invalid: They have an ID, so they cannot be treated as\n"),n("a",{attrs:{href:"https://www.jsonrpc.org/specification#notification",target:"_blank",rel:"noopener noreferrer"}},[e._v("notifications"),n("OutboundLink")],1),e._v("; but the ID corresponds to a request that was\nalready completed.  In practice, this means that general-purpose JSON-RPC\nlibraries cannot use this method correctly -- it requires a custom client.")]),e._v(" "),n("p",[e._v("The Go RPC client from the Tendermint core can support this case, but clients\nin other languages have no easy solution.")]),e._v(" "),n("p",[e._v("This is the cause of issue "),n("a",{attrs:{href:"https://github.com/tendermint/tendermint/issues/2949",target:"_blank",rel:"noopener noreferrer"}},[e._v("#2949"),n("OutboundLink")],1),e._v(".")])]),e._v(" "),n("li",[n("p",[n("strong",[e._v("Subscriptions are terminated by disconnection.")]),e._v(" When the connection to the\nclient is interrupted, the subscription is silently dropped.")]),e._v(" "),n("p",[e._v("This is a reasonable behavior, but it matters because a client whose\nsubscription is dropped gets no useful error feedback, just a closed\nconnection.  Should they try again?  Is the node overloaded?  Was the client\ntoo slow?  Did the caller forget to respond to pings? Debugging these kinds\nof failures is unnecessarily painful.")]),e._v(" "),n("p",[e._v("Websockets compound this, because websocket connections time out if no\ntraffic is seen for a while, and keeping them alive requires active\ncooperation between the client and server.  With a plain TCP socket, liveness\nis handled transparently by the keepalive mechanism.  On a websocket,\nhowever, one side has to occasionally send a PING (if the connection is\notherwise idle).  The other side must return a matching PONG in time, or the\nconnection is dropped.  Apart from being tedious, this is highly susceptible\nto CPU load.")]),e._v(" "),n("p",[e._v("The Tendermint Go implementation automatically sends and responds to pings.\nClients in other languages (or not wanting to use the Tendermint libraries)\nneed to handle it explicitly.  This burdens the client for no practical\nbenefit: A subscriber has no information about when matching events may be\navailable, so it shouldn't have to participate in keeping the connection\nalive.")])]),e._v(" "),n("li",[n("p",[n("strong",[e._v("Mismatched load profiles.")]),e._v(" Most of the RPC service is mainly important for\nlow-volume local use, either by the application the node serves (e.g., the\nABCI methods) or by the node operator (e.g., the info methods).  Event\nsubscription is important for remote clients, and may represent a much higher\nvolume of traffic.")]),e._v(" "),n("p",[e._v("This matters because both are using the same JSON-RPC mechanism. For\nlow-volume local use, the ergonomics of JSON-RPC are a good fit: It's easy to\nissue queries from the command line (e.g., using "),n("code",[e._v("curl")]),e._v(") or to write scripts\nthat call the RPC methods to monitor the running node.")]),e._v(" "),n("p",[e._v("For high-volume remote use, JSON-RPC is not such a good fit: Even leaving\naside the non-standard delivery protocol mentioned above, the time and memory\ncost of encoding event data matters for the stability of the node when there\ncan be potentially hundreds of subscribers. Moreover, a subscription is\nlong-lived compared to most RPC methods, in that it may persist as long the\nnode is active.")])]),e._v(" "),n("li",[n("p",[n("strong",[e._v("Mismatched security profiles.")]),e._v(" The RPC service exports several methods\nthat should not be open to arbitrary remote callers, both for correctness\nreasons (e.g., "),n("code",[e._v("remove_tx")]),e._v(" and "),n("code",[e._v("broadcast_tx_*")]),e._v(") and for operational\nstability reasons (e.g., "),n("code",[e._v("tx_search")]),e._v("). A node may still need to expose\nevents, however, to support UI tools.")]),e._v(" "),n("p",[e._v("This matters, because all the methods share the same network endpoint. While\nit is possible to block the top-level GET and POST handlers with a proxy,\nexposing the "),n("code",[e._v("/websocket")]),e._v(" handler exposes not "),n("em",[e._v("only")]),e._v(" the event subscription\nmethods, but the rest of the service as well.")])])]),e._v(" "),n("h3",{attrs:{id:"possible-improvements"}},[n("a",{staticClass:"header-anchor",attrs:{href:"#possible-improvements"}},[e._v("#")]),e._v(" Possible Improvements")]),e._v(" "),n("p",[e._v("There are several things we could do to improve the experience of developers\nwho need to subscribe to events from the consensus node. These are not all\nmutually exclusive.")]),e._v(" "),n("ol",[n("li",[n("p",[n("strong",[e._v("Split event subscription into a separate service")]),e._v(". Instead of exposing\nevent subscription on the same endpoint as the rest of the RPC service,\ndedicate a separate endpoint on the node for "),n("em",[e._v("only")]),e._v(" event subscription.  The\nrest of the RPC services ("),n("em",[e._v("sans")]),e._v(" events) would remain as-is.")]),e._v(" "),n("p",[e._v("This would make it easy to disable or firewall outside access to sensitive\nRPC methods, without blocking access to event subscription (and vice versa).\nThis is probably worth doing, even if we don't take any of the other steps\ndescribed here.")])]),e._v(" "),n("li",[n("p",[n("strong",[e._v("Use a different protocol for event subscription.")]),e._v(" There are various ways\nwe could approach this, depending how much we're willing to shake up the\ncurrent API. Here are sketches of a few options:")]),e._v(" "),n("ul",[n("li",[n("p",[e._v("Keep the websocket, but rework the API to be more JSON-RPC compliant,\nperhaps by converting event delivery into notifications.  This is less\nup-front change for existing clients, but retains all of the existing\nimplementation complexity, and doesn't contribute much toward more serious\nperformance and UX improvements later.")])]),e._v(" "),n("li",[n("p",[e._v("Switch from websocket to plain HTTP, and rework the subscription API to\nuse a more conventional request/response pattern instead of streaming.\nThis is a little more up-front work for existing clients, but leverages\nbetter library support for clients not written in Go.")]),e._v(" "),n("p",[e._v('The protocol would become more chatty, but we could mitigate that with\nbatching, and in return we would get more control over what to do about\nslow clients: Instead of simply silently dropping them, as we do now, we\ncould drop messages and signal the client that they missed some data ("M\ndropped messages since your last poll").')]),e._v(" "),n("p",[e._v("This option is probably the best balance between work, API change, and\nbenefit, and has a nice incidental effect that it would be easier to debug\nsubscriptions from the command-line, like the other RPC methods.")])]),e._v(" "),n("li",[n("p",[e._v("Switch to gRPC: Preserves a persistent connection and gives us a more\nefficient binary wire format (protobuf), at the cost of much more work for\nclients and harder debugging. This may be the best option if performance\nand server load are our top concerns.")]),e._v(" "),n("p",[e._v("Given that we are currently using JSON-RPC, however, I'm not convinced the\ncosts of encoding and sending messages on the event subscription channel\nare the limiting factor on subscription efficiency, however.")])])])]),e._v(" "),n("li",[n("p",[n("strong",[e._v("Delegate event subscriptions to a proxy.")]),e._v(" Give responsibility for\nmanaging event subscription to a proxy that runs separately from the node,\nand switch the node to push events to the proxy (like a webhook) instead of\nserving subscribers directly.  This is more work for the operator (another\nprocess to configure and run) but may scale better for big networks.")]),e._v(" "),n("p",[e._v("I mention this option for completeness, but making this change would be a\nfairly substantial project.  If we want to consider shifting responsibility\nfor event subscription outside the node anyway, we should probably be more\nsystematic about it. For a more principled approach, see point (4) below.")])]),e._v(" "),n("li",[n("p",[n("strong",[e._v("Move event subscription downstream of indexing.")]),e._v(" We are already planning\nto give applications more control over event indexing. By extension, we\nmight allow the application to also control how events are filtered,\nqueried, and subscribed. Having the application control these concerns,\nrather than the node, might make life easier for developers building UI and\ntools for that application.")]),e._v(" "),n("p",[e._v("This is a much larger change, so I don't think it is likely to be practical\nin the near-term, but it's worth considering as a broader option. Some of\nthe existing code for filtering and selection could be made more reusable,\nso applications would not need to reinvent everything.")])])]),e._v(" "),n("h2",{attrs:{id:"references"}},[n("a",{staticClass:"header-anchor",attrs:{href:"#references"}},[e._v("#")]),e._v(" References")]),e._v(" "),n("ul",[n("li",[n("a",{attrs:{href:"https://docs.tendermint.com/master/rpc/",target:"_blank",rel:"noopener noreferrer"}},[e._v("Tendermint RPC service"),n("OutboundLink")],1)]),e._v(" "),n("li",[n("a",{attrs:{href:"https://github.com/tendermint/tendermint/blob/master/internal/rpc/core/routes.go#L12",target:"_blank",rel:"noopener noreferrer"}},[e._v("Tendermint RPC routes"),n("OutboundLink")],1)]),e._v(" "),n("li",[n("a",{attrs:{href:"./rfc-005-event-system.rst"}},[e._v("Discussion of the event system")])]),e._v(" "),n("li",[n("RouterLink",{attrs:{to:"/rfc/rfc-002-ipc-ecosystem.html#rpc-transport"}},[e._v("Discussion about RPC transport options")]),e._v(" (from RFC 002)")],1),e._v(" "),n("li",[n("a",{attrs:{href:"https://datatracker.ietf.org/doc/html/rfc6455",target:"_blank",rel:"noopener noreferrer"}},[e._v("RFC 6455: The websocket protocol"),n("OutboundLink")],1)]),e._v(" "),n("li",[n("a",{attrs:{href:"https://www.jsonrpc.org/specification",target:"_blank",rel:"noopener noreferrer"}},[e._v("JSON-RPC 2.0 Specification"),n("OutboundLink")],1)])])])}),[],!1,null,null,null);t.default=s.exports}}]);