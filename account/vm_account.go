package account

import (
	"github.com/tendermint/tendermint/vm"
)

func (acc *VmAccount) GetAddress() vm.Word         { return acc.Address }
func (acc *VmAccount) GetBalance() uint64          { return acc.Balance }
func (acc *VmAccount) GetCode() []byte             { return acc.Code }
func (acc *VmAccount) GetNonce() uint64            { return acc.Nonce }
func (acc *VmAccount) GetStorageRoot() vm.Word     { return acc.StorageRoot }
func (acc *VmAccount) SetAddress(addr vm.Word)     { acc.Address = addr }
func (acc *VmAccount) SetBalance(balance uint64)   { acc.Balance = balance }
func (acc *VmAccount) SetCode(code []byte)         { acc.Code = code }
func (acc *VmAccount) SetNonce(nonce uint64)       { acc.Nonce = nonce }
func (acc *VmAccount) SetStorageRoot(root vm.Word) { acc.StorageRoot = root }

//-----------------------------------------------------------------------------

type VmAccount struct {
	Address     vm.Word
	Balance     uint64
	Code        []byte
	Nonce       uint64
	StorageRoot vm.Word

	// not needed for vm but required for sanity
	PubKey PubKey
}
