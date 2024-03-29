/*
   ____             _              _                      _____           _                  _
  / __ \           | |            | |                    |  __ \         | |                | |
 | |  | |_ __   ___| |     ___  __| | __ _  ___ _ __     | |__) | __ ___ | |_ ___   ___ ___ | |
 | |  | | '_ \ / _ \ |    / _ \/ _` |/ _` |/ _ \ '__|    |  ___/ '__/ _ \| __/ _ \ / __/ _ \| |
 | |__| | | | |  __/ |___|  __/ (_| | (_| |  __/ |       | |   | | | (_) | || (_) | (_| (_) | |
  \____/|_| |_|\___|______\___|\__,_|\__, |\___|_|       |_|   |_|  \___/ \__\___/ \___\___/|_|
                                      __/ |
                                     |___/


Copyright 2017 - 2019 OneLedger
*/

package balance

import (
	"github.com/Oneledger/protocol/data/keys"
	"github.com/Oneledger/protocol/serialize"
	"github.com/Oneledger/protocol/storage"
	"github.com/pkg/errors"
	"strings"
)

type Store struct {
	State  *storage.State
	prefix []byte
}

func NewStore(prefix string, state *storage.State) *Store {
	return &Store{
		State:  state,
		prefix: storage.Prefix(prefix),
	}
}

func (st *Store) WithState(state *storage.State) *Store {
	st.State = state
	return st
}

func (st *Store) get(key storage.StoreKey) (amt *Amount, err error) {
	prefixed := append(st.prefix, storage.StoreKey(key)...)
	dat, _ := st.State.Get(prefixed)
	amt = NewAmount(0)
	if len(dat) == 0 {
		return
	}
	err = serialize.GetSerializer(serialize.PERSISTENT).Deserialize(dat, amt)
	return
}

func (st *Store) set(key storage.StoreKey, amt Amount) error {
	dat, err := serialize.GetSerializer(serialize.PERSISTENT).Serialize(amt)
	if err != nil {
		return err
	}

	prefixed := append(st.prefix, key...)
	err = st.State.Set(prefixed, dat)
	return err
}

func (st *Store) iterate(addr keys.Address, fn func(c string, amt Amount) bool) bool {
	return st.State.IterateRange(
		append(st.prefix, addr...),
		storage.Rangefix(string(append(st.prefix, addr...))),
		true,
		func(key, value []byte) bool {
			amt := NewAmount(0)
			err := serialize.GetSerializer(serialize.PERSISTENT).Deserialize(value, amt)
			if err != nil {
				return true
			}
			arr := strings.Split(string(key), storage.DB_PREFIX)
			return fn(arr[len(arr)-1], *amt)
		},
	)
}

// todo: add back if necessary. address will not work because key will be address+currency
//func (st *Store) Exists(address keys.Address) bool {
//	key := append(st.prefix, storage.StoreKey(address)...)
//	return st.State.Exists(key)
//}

func (st *Store) AddToAddress(addr keys.Address, coin Coin) error {
	key := storage.StoreKey(string(addr) + storage.DB_PREFIX + coin.Currency.Name)

	amt, err := st.get(key)
	if err != nil {
		return errors.Wrapf(err, "failed to get address balance %s", addr.String())
	}

	base := coin.Currency.NewCoinFromAmount(*amt)
	newCoin := base.Plus(coin)

	return st.set(key, *newCoin.Amount)
}

func (st *Store) MinusFromAddress(addr keys.Address, coin Coin) error {
	key := storage.StoreKey(string(addr) + storage.DB_PREFIX + coin.Currency.Name)

	amt, err := st.get(key)
	if err != nil {
		return errors.Wrapf(err, "failed to get address balance %s", addr.String())
	}

	base := coin.Currency.NewCoinFromAmount(*amt)
	newCoin, err := base.Minus(coin)
	if err != nil {
		return errors.Wrap(err, "minus from address")
	}

	return st.set(key, *newCoin.Amount)
}

func (st *Store) CheckBalanceFromAddress(addr keys.Address, coin Coin) error {
	key := storage.StoreKey(string(addr) + storage.DB_PREFIX + coin.Currency.Name)

	amt, err := st.get(key)
	if err != nil {
		return errors.Wrapf(err, "failed to get address balance %s", addr.String())
	}

	base := coin.Currency.NewCoinFromAmount(*amt)
	_, err = base.Minus(coin)
	if err != nil {
		return errors.Wrap(err, "minus from address")
	}

	return nil
}

func (st *Store) GetBalance(address keys.Address, list *CurrencySet) (balance *Balance, err error) {
	balance = NewBalance()
	st.iterate(address, func(c string, amt Amount) bool {
		currency, ok := list.GetCurrencyByName(c)
		if !ok {
			err = errors.New("currency not expected")
			return false
		}
		balance.setCoin(currency.NewCoinFromAmount(amt))

		return false
	})
	return
}
