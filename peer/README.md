## Channel ""

<table>
  <tr>
    <td><b>Filter</b></td>
    <td>None</td>
  </tr>
  <tr>
    <td><b>Message</b></td>
    <td>
      <ul>
        <li>RefreshFilterMsg</li>
        <li>PeerExchangeMsg</li>
      </ul>
    </td>
  </tr>
</table>


## Channel "block"

<table>
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


## Channel "mempool"

<table>
  <tr>
    <td><b>Filter</b></td>
    <td>
      Bloom filter (n:10k, p:0.02 -> k:6, m:10KB)<br/>
      Refreshes every new block
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


## Channel "consensus"                                                                                                                                                                                                                        
<table>
  <tr>
    <td><b>Filter</b></td>
    <td>
      Bitarray filter<br/>
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




