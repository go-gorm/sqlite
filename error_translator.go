package sqlite

import (
	"encoding/json"
	"gorm.io/gorm"
)

var errCodes = map[string]int{
	"uniqueConstraint": 2067,
}

type ErrMessage struct {
	Code         int `json:"Code"`
	ExtendedCode int `json:"ExtendedCode"`
	SystemErrno  int `json:"SystemErrno"`
}

func (dialector Dialector) Translate(err error) error {
	parsedErr, marshalErr := json.Marshal(err)
	if marshalErr != nil {
		return err
	}

	var errMsg ErrMessage
	unmarshalErr := json.Unmarshal(parsedErr, &errMsg)
	if unmarshalErr != nil {
		return err
	}

	if errMsg.ExtendedCode == errCodes["uniqueConstraint"] {
		return gorm.ErrDuplicatedKey
	}

	return err
}
