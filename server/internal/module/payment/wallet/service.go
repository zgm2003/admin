package wallet

// Wallet reads and transaction participants intentionally live in separate
// layers: HTTP callers can only read a wallet, while settlement owns mutation.
