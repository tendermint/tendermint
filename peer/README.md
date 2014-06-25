## Channel ""

<table>
  <tr>
    <th>Filter</th>
    <td>None</td>
  </tr>
  <tr>
    <th>Messages</th>
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
    <th>Filter</th>
    <td>Custom</td>
  </tr>
  <tr>
    <th>Messages</th>
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
    <th>Filter</th>
    <td>
      Bloom filter (n:10k, p:0.02 -> k:6, m:10KB)<br/>
      Refreshes every new block
    </td>
  </tr>
  <tr>
    <th>Messages</th>
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
    <th>Filter</th>
    <td>
      Bitarray filter<br/>
      Refreshes every new consensus round
    </td>
  </tr>
  <tr>
    <th>Messages</th>
    <td>
      <ul>
        <li>ProposalMsg</li>
        <li>VoteMsg</li>
        <li>NewBlockMsg</li>
      </ul>
    </td>
  </tr>
</table>




