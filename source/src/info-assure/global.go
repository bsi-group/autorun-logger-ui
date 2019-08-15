package main

const ALERT_YELLOW string = `<div class="alert alert-warning" role="alert">%v</div>`
const ALERT_RED string = `<div class="alert alert-danger" role="alert">%v</div>`
const ALERT_GREEN string = `<div class="alert alert-success" role="alert">%v</div>`

type AccountType int16

const (
	USER  AccountType = 0
	ADMIN AccountType = 1
)

func (at AccountType) String() string {

	names := [...]string{"User", "Admin"}

	if at < USER || at > ADMIN {
		return "Unknown"
	}

	return names[at]
}

const (
	EXPORT_TYPE_SHA256 = 1
	EXPORT_TYPE_MD5    = 2
	EXPORT_TYPE_DOMAIN = 3
	EXPORT_TYPE_HOST   = 4
)

const (
	SEARCH_TYPE_FILE_PATH     = 1
	SEARCH_TYPE_LAUNCH_STRING = 2
	SEARCH_TYPE_LOCATION      = 3
	SEARCH_TYPE_ITEM_NAME     = 4
	SEARCH_TYPE_PROFILE       = 5
	SEARCH_TYPE_DESCRIPTION   = 6
	SEARCH_TYPE_COMPANY       = 7
	SEARCH_TYPE_SIGNER        = 8
	SEARCH_TYPE_SHA256        = 9
	SEARCH_TYPE_MD5           = 10
)

const (
	DATA_TYPE_ALERTS   = 1
	DATA_TYPE_AUTORUNS = 2
)

const (
	VERIFIED_ALL   = 0
	VERIFIED_TRUE  = 1
	VERIFIED_FALSE = 2
	VERIFIED_MS    = 3
)
