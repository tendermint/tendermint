_UPDATE: This page is outdated.  Secure Native contracts have been implemented, though not "DestroyStake()".  That's not a bad idea._

Chain modification and control is accessed through secure native contracts (SNC here for short).

Each SNC should have an associated permission stored with the account which controls access to that contract. The format of the Native contract permission should be identical to the base chain permissions described in [the Permissions Wiki page](https://github.com/tendermint/tendermint/wiki/Permissions).

In order to access each SNC and perform its action the caller must have either the correct SNC perm or the Root base permission.


## Secure Native Contracts we need (names subject to bettering):

**getPermValue(permname, account)**  
Returns: the permission value for the provided account, permname pair  
Default global SNC perm value (GPV): True (Everyone should be able to access this perm by default as it is informational only)  

**setPermValue(permname, account, value)** - Sets permission value for account for permission permname.  
Returns: 0 - Failure (lack of permissions, perm doesn't exist etc) 1 - permission successfully set  
Default GPV: False  

**unsetPerm(permname, account)** - Set the "set/unset" bit to "unset" - restoring the global value to the account  
Returns: 0 - Failure, 1 - permission successfully set  
Default GPV: False 

**setPermGlobalValue(permname, value)** - Sets global value for permission permname   
Returns: 0 - Failure to complete, 1 - Successfully set  
Default GPV: False  

**clearPerm(permname)** - Resets all accounts to "unset" for permission permname
Returns: 0 - Failure to complete, 1 - Successful
Default GPV: False

---------
If general "dynamic" perm creation allowed the above should work for those as well as the following

**addPerm(permname)** - Creates new "dynamic" perm  
Returns: 0 - Failure, 1 -  Success  
Default GPV: False  

**rmPerm(permname)** - Removes a "dynamic" perm (Optional)  
Returns: 0 - Failure, 1 -  Success  
Default GPV: False  

----------
Others

If we are building this lovely framework for secure access to chain properties we might as well expose some other core abilities right?

for example:

**deleteStake(account)** - Remove all stake (balance? not sure about the proper term) owned by account (adjust total stake accordingly)  

**setStake(account)** - allows setting to an arbitrary value.  

etc. Not sure there are that many more. But anything that someone might want to control the in course of administrating their chain should be considered for addition to this list. also any informational aspects not normally accessible through vm opcodes.