## Channels

Each peer connection is multiplexed into channels.  Each channel can optionally have an associated filter which determines whether the peer already knows of the message.  The system is designed to be easily extensible for various applications.
<hr />

### Default channel

The default channel is used to communicate state changes, pings, peer exchange, and other automatic internal messages that all P2P protocols would want implemented.

<table>
  <tr>
    <td><b>Channel</b></td>
    <td>""</td>
  </tr>
  <tr>
    <td><b>Filter</b></td>
    <td>None<br/>Messages in this channel is not filtered.</td>
  </tr>
  <tr>
    <td><b>Messages</b></td>
    <td>
      <ul>
        <li>PingMsg/PongMsg</li>
        <li>PeerExchangeMsg</li>
        <li>RefreshFilterMsg</li>
      </ul>
    </td>
  </tr>
</table>
<hr />

### Block channel

The block channel is used to propagate block or header information to new peers or peers catching up with the blockchain.

<table>
  <tr>
    <td><b>Channel</b></td>
    <td>"block"</td>
  </tr>
  <tr>
    <td><b>Filter</b></td>
    <td>
      Custom<br/>
      Nodes should only advertise having a header or block at height 'h' if it also has all the headers or blocks less than 'h'.  Thus this filter need only keep track of two integers -- one for the most recent header height 'h_h' and one for the most recent block height 'h_b', where 'h_b' &lt;= 'h_h'.
    </td>
  </tr>
  <tr>
    <td><b>Messages</b></td>
    <td>
      <ul>
        <li>RequestMsg</li>
        <li>BlockMsg</li>
        <li>HeaderMsg</li>
      </ul>
    </td>
  </tr>
</table>
<hr />

### Mempool channel

The mempool channel is used for broadcasting new transactions that haven't yet entered the blockchain.  It uses a lossy bloom filter on either end, but with sufficient fanout and filter nonce updates every new block, all transactions will eventually reach every node.

<table>
  <tr>
    <td><b>Channel</b></td>
    <td>"mempool"</td>
  </tr>
  <tr>
    <td><b>Filter</b></td>
    <td>
      Bloom filter (n:10k, p:0.02 -> k:6, m:10KB)<br/>
      Each peer's filter has a random nonce that scrambles the message hashes<br/>
      The filter & nonce refreshes every new block<br/>
    </td>
  </tr>
  <tr>
    <td><b>Messages</b></td>
    <td>
      <ul>
        <li>MempoolTxMsg</li>
      </ul>
    </td>
  </tr>
</table>
<hr />

### Consensus channel

The consensus channel broadcasts all information used in the rounds of the Tendermint consensus mechanism.

<table>
  <tr>
    <td><b>Channel</b></td>
    <td>"consensus"</td>
  </tr>
  <tr>
    <td><b>Filter</b></td>
    <td>
      Bitarray filter<br/>
      Each validator has a predetermined index in teh bitarray<br/>
      Refreshes every new consensus round
    </td>
  </tr>
  <tr>
    <td><b>Messages</b></td>
    <td>
      <ul>
        <li>ProposalMsg</li>
        <li>VoteMsg</li>
        <li>NewBlockMsg</li>
      </ul>
    </td>
  </tr>
</table>
