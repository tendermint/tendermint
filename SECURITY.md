# Security

## Reporting a Bug

As part of our [Coordinated Vulnerability Disclosure
Policy](https://tendermint.com/security), we operate a [bug
bounty](https://hackerone.com/tendermint).
See the policy for more details on submissions and rewards, and see "Example Vulnerabilities" (below) for examples of the kinds of bugs we're most interested in.

### Guidelines 

We require that all researchers:

* Use the bug bounty to disclose all vulnerabilities, and avoid posting vulnerability information in public places, including Github Issues, Discord channels, and Telegram groups
* Make every effort to avoid privacy violations, degradation of user experience, disruption to production systems (including but not limited to the Cosmos Hub), and destruction of data
* Keep any information about vulnerabilities that you’ve discovered confidential between yourself and the Tendermint Core engineering team until the issue has been resolved and disclosed 
* Avoid posting personally identifiable information, privately or publicly

If you follow these guidelines when reporting an issue to us, we commit to:

* Not pursue or support any legal action related to your research on this vulnerability
* Work with you to understand, resolve and ultimately disclose the issue in a timely fashion 

## Disclosure Process 

Tendermint Core uses the following disclosure process:

1. Once a security report is received, the Tendermint Core team works to verify the issue and confirm its severity level using CVSS. 
2. The Tendermint Core team collaborates with the Gaia team to determine the vulnerability’s potential impact on the Cosmos Hub. 
3. Patches are prepared for eligible releases of Tendermint in private repositories. See “Supported Releases” below for more information on which releases are considered eligible. 
4. If it is determined that a CVE-ID is required, we request a CVE through a CVE Numbering Authority. 
5. We notify the community that a security release is coming, to give users time to prepare their systems for the update. Notifications can include forum posts, tweets, and emails to partners and validators, including emails sent to the [Tendermint Security Mailing List](https://berlin.us4.list-manage.com/subscribe?u=431b35421ff7edcc77df5df10&id=3fe93307bc).
6. 24 hours following this notification, the fixes are applied publicly and new releases are issued. 
7. Cosmos SDK and Gaia update their Tendermint Core dependencies to use these releases, and then themselves issue new releases. 
8. Once releases are available for Tendermint Core, Cosmos SDK and Gaia, we notify the community, again, through the same channels as above. We also publish a Security Advisory on Github and publish the CVE, as long as neither the Security Advisory nor the CVE include any information on how to exploit these vulnerabilities beyond what information is already available in the patch itself. 
9. Once the community is notified, we will pay out any relevant bug bounties to submitters. 
10. One week after the releases go out, we will publish a post with further details on the vulnerability as well as our response to it. 

This process can take some time. Every effort will be made to handle the bug in as timely a manner as possible, however it's important that we follow the process described above to ensure that disclosures are handled consistently and to keep Tendermint Core and its downstream dependent projects--including but not limited to Gaia and the Cosmos Hub--as secure as possible. 

### Example Timeline 

The following is an example timeline for the triage and response. The required roles and team members are described in parentheses after each task; however, multiple people can play each role and each person may play multiple roles. 

#### > 24 Hours Before Release Time

1. Request CVE number (ADMIN) 
2. Gather emails and other contact info for validators (COMMS LEAD) 
3. Test fixes on a testnet  (TENDERMINT ENG, COSMOS ENG) 
4. Write “Security Advisory” for forum (TENDERMINT LEAD) 

#### 24 Hours Before Release Time

1. Post “Security Advisory” pre-notification on forum (TENDERMINT LEAD) 
2. Post Tweet linking to forum post (COMMS LEAD) 
3. Announce security advisory/link to post in various other social channels (Telegram, Discord) (COMMS LEAD) 
4. Send emails to validators or other users (PARTNERSHIPS LEAD) 

#### Release Time

1. Cut Tendermint releases for eligible versions (TENDERMINT ENG, TENDERMINT LEAD)
2. Cut Cosmos SDK release for eligible versions (COSMOS ENG)
3. Cut Gaia release for eligible versions (GAIA ENG)
4. Post “Security releases” on forum (TENDERMINT LEAD)
5. Post new Tweet linking to forum post (COMMS LEAD)
6. Remind everyone via social channels (Telegram, Discord)  that the release is out (COMMS LEAD)
7. Send emails to validators or other users (COMMS LEAD) 
8. Publish Security Advisory and CVE, if CVE has no sensitive information (ADMIN) 

#### After Release Time

1. Write forum post with exploit details (TENDERMINT LEAD)
2. Approve pay-out on HackerOne for submitter (ADMIN) 

#### 7 Days After Release Time

1. Publish CVE if it has not yet been published (ADMIN) 
2. Publish forum post with exploit details (TENDERMINT ENG, TENDERMINT LEAD)

## Supported Releases

The Tendermint Core team commits to releasing security patch releases for both the latest minor release as well for the major/minor release that the Cosmos Hub is running. 

If you are running older versions of Tendermint Core, we encourage you to upgrade at your earliest opportunity so that you can receive security patches directly from the Tendermint repo. While you are welcome to backport security patches to older versions for your own use, we will not publish or promote these backports. 

## Scope

The full scope of our bug bounty program is outlined on our [Hacker One program page](https://hackerone.com/tendermint). Please also note that, in the interest of the safety of our users and staff, a few things are explicitly excluded from scope:

* Any third-party services 
* Findings from physical testing, such as office access 
* Findings derived from social engineering (e.g., phishing)

## Example Vulnerabilities 

The following is a list of examples of the kinds of vulnerabilities that we’re most interested in. It is not exhaustive: there are other kinds of issues we may also be interested in! 

### Specification

* Conceptual flaws
* Ambiguities, inconsistencies, or incorrect statements
* Mis-match between specification and implementation of any component

### Consensus

Assuming less than 1/3 of the voting power is Byzantine (malicious):

* Validation of blockchain data structures, including blocks, block parts, votes, and so on
* Execution of blocks
* Validator set changes
* Proposer round robin
* Two nodes committing conflicting blocks for the same height (safety failure)
* A correct node signing conflicting votes
* A node halting (liveness failure)
* Syncing new and old nodes


### Networking

* Authenticated encryption (MITM, information leakage)
* Eclipse attacks
* Sybil attacks
* Long-range attacks
* Denial-of-Service

### RPC

* Write-access to anything besides sending transactions
* Denial-of-Service
* Leakage of secrets

### Denial-of-Service

Attacks may come through the P2P network or the RPC layer:

* Amplification attacks
* Resource abuse
* Deadlocks and race conditions

### Libraries

* Serialization (Amino)
* Reading/Writing files and databases

### Cryptography

* Elliptic curves for validator signatures
* Hash algorithms and Merkle trees for block validation
* Authenticated encryption for P2P connections

### Light Client

* Core verification 
* Bisection/sequential algorithms
