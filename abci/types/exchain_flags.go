package types

import "github.com/spf13/viper"

const FlagCloseMutex = "close-mutex"

func GetCloseMutex() bool {
	return viper.GetBool(FlagCloseMutex)
}
