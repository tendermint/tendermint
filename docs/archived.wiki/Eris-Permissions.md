Design goals:
- permissions for all basic chain actions
- native implementations for efficiency
- permissions over a set of administrative native contracts
- GlobalPermissions defaults for accounts with "unset" permission values
- customizable set of "roles" for defining capabilities

Every account on a chain has a set of BasePermissions:

```
type PermFlag uint64
Root PermFlag = 1 << iota # Do anything (!)
Send             # SendTx, CALL to non-contract account
Call             # CallTx, CALL to a contract
CreateContract   # CallTx to empty address, CREATE
CreateAccount    # SendTx to non-existing account, CALL to non-existing account, (... ?)
Bond             # BondTx, participate in the consensus
Name             # NameTx, register in the name registry

HasBase          # check if an account has permission
SetBase          # set a permission for an account
UnsetBase        # unset a permission for an account
SetGlobal        # set a global permission
HasRole          # check if an account has a role
AddRole          # add a role to an account
RmRole           # rm a role from an account
```

Root gives you every permission - it should only be used for development and for initializing a production chain. 

The HasBase and HasRole permissions allow us to isolate contracts. Maybe they're unnecessary.

The `genesis.json` specifies some initial accounts, along with their permissions. It also specifies the "global permissions", which are stored in account with address 0x00000000000000000000, and which are given by default to new accounts.

Each account exposes a permission struct:

```
type Account struct{
  ...
  Permissions *AccountPermissions 
}
```

Account Permissions consists of Base permissions (which cover basic interaction with the chain and the secure native contracts) and a set of Roles.

```
// Permissions for every account determining basic interaction with chain
type BasePermissions struct{
  Flags uint64
  SetBit uint64 // unset bits default to global values
}

BasePermissions by itself has no roles.

// Permissions for administrative personnel with the same set of BasePermissions
// and arbitrary additional permissions on top.
type AccountPermissions struct{
  Base BasePermissions
  Roles []string
}
```

Subtleties:

- since all permission updates are done through transactions in the consensus, bonded validators have a final say in everything (even root)
- new accounts are created either by SendTx or CALL (opcode) to an unknown address. For SendTx, all inputs must have both Send permission and CreateAccount permission. For CALL, the contract must have both Call permission and CreateAccount permission
- tokens are still a thing, there is gas, value transfers, etc. this is probably not terrible and can be used as general purpose tracking of activity on the network, where periodic refills via some native contract may be necessary.




