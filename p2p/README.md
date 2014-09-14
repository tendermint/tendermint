# P2P Module

P2P provides an abstraction around peer-to-peer communication.<br/>
Communication happens via Reactors that react to messages from peers.<br/>
Each Reactor has one or more Channels of communication for each Peer.<br/>
Channels are multiplexed automatically and can be configured.<br/>
A Switch is started upon app start, and handles Peer management.<br/>
A PEXReactor implementation is provided to automate peer discovery.<br/>

## Channels

Each peer connection is multiplexed into channels.
The p2p module comes with a channel implementation used for peer
discovery (called PEX, short for "peer exchange").

<table>
  <tr>
    <td><b>Channel</b></td>
    <td>"PEX"</td>
  </tr>
  <tr>
    <td><b>Messages</b></td>
    <td>
      <ul>
        <li>pexRequestMsg</li>
        <li>pexResponseMsg</li>
      </ul>
    </td>
  </tr>
</table>
<hr />

## Resources

* http://www.upnp-hacks.org/upnp.html
