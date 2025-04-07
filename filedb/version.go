package filedb

import (
	"fmt"
)

// Breaking changes - The database structure has changed & must be migrated. I.E New required field, field type chaning, adding NOT NULL on a previously NULL column.
const MajorVersion int = 3

// Changes that may change what values can be added and may make some values invalid, but the strucutre is the same. I.E Adding UNIQUE on a value, adding a new CHECK constraint, or
// changes to the backend stuff that is largely abstracted. I.E db_info table
const MinorVersion int = 2

// Bug fixes to the Go code that do not impact how the database works, but change now the go code interacts with it, but no changes in the database.
const Revision int = 1

// Version code name
const VersionCodeName string = "EcstacyInGrief"

func FormatVersion(major, minor, revision int) string {
	return fmt.Sprintf("%d.%dr%d", major, minor, revision)
}

func MajorVersionToCodeName(major int) string {
	switch major {
	case 1:
		// Erase Me - Make Them Suffer
		return "EraseMe"
	case 2:
		// In The House Of Leaves - Ghost Atlas
		return "InTheHouseOfLeaves"
	case 3:
		// Ecstacy In Grief - Cane Hill
		return "EcstacyInGrief"
	case 4:
		// West Coast - Lana Del Rey
		return "WestCoast"
	default:
		return fmt.Sprintf("<Unsupported version '%d'>", major)
	}
}
