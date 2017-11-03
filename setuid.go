package goservicetools

import (
	"fmt"
	"os/user"
	"strconv"

	"github.com/ilya1st/configuration-go"
)

// SetuidData intended to transmit data to AppStop to set credentials there
type SetuidData struct {
	uid uint32
	gid uint32 // -1 if do not need
}

// This file intended store setuid functions for work

// GetSetUIDGIDData lookups Setuid gid data to suid
// no tests here cause too system dependent
func GetSetUIDGIDData(setuidConf configuration.IConfig) (*SetuidData, error) {
	err := CheckSetuidConfig(setuidConf)
	if err != nil {
		return nil, err
	}
	setuid, err := setuidConf.GetBooleanValue("setuid")
	if err != nil { // if all is too bad
		panic(fmt.Errorf("Setuid configuration check failed. %v", err))
	}
	if !setuid {
		return nil, nil
	}
	username, err := setuidConf.GetStringValue("user")
	if err != nil { // if all is too bad
		panic(fmt.Errorf("Setuid configuration check failed. %v", err))
	}
	groupname, err := setuidConf.GetStringValue("group")
	if err != nil { // if all is too bad
		panic(fmt.Errorf("Setuid configuration check failed. %v", err))
	}
	if username == "" {
		panic(fmt.Errorf("Setuid configuration check failed. Empty user name"))
	}
	userStruct, err := user.Lookup(username)
	if err != nil {
		logger := GetSystemLogger()
		err := fmt.Errorf("SetUIDGID: user %s lookup error: %v", username, err)
		if logger != nil {
			logger.Fatal().Msgf("%v", err)
		} else {
			panic(err)
		}
	}
	var groupStruct *user.Group
	groupStruct = nil
	if groupname != "" {
		groupStruct, err = user.LookupGroup(groupname)
		if err != nil {
			logger := GetSystemLogger()
			err := fmt.Errorf("SetUIDGID: group %s lookup error: %v", groupname, err)
			if logger != nil {
				logger.Fatal().Msgf("%v", err)
			} else {
				panic(err)
			}
		}
	}
	uid, err := strconv.ParseInt(userStruct.Uid, 10, 64)
	if err != nil {
		panic(fmt.Errorf("Can not parse user id %s: %v", userStruct.Uid, err))
	}
	var gid int64
	gidset := false
	if groupStruct != nil {
		gid, err = strconv.ParseInt(groupStruct.Gid, 10, 64)
		if err != nil {
			panic(fmt.Errorf("Can not parse group id %s: %v", groupStruct.Gid, err))
		}
		gidset = true
	} else {
		if userStruct.Gid == "" {
			gidset = false
		} else {
			gid, err = strconv.ParseInt(userStruct.Gid, 10, 64)
			if err != nil {
				panic(fmt.Errorf("Can not parse group id %s: %v", userStruct.Gid, err))
			}
			gidset = true
		}
	}
	if gidset {
		return &SetuidData{uid: uint32(uid), gid: uint32(gid)}, nil
	}
	return &SetuidData{uid: uint32(uid), gid: 0}, nil
}
