package filedb

import (
	"fmt"
)

type migrationDb struct {
	f *FileDb
}

func (m *migrationDb) updateRevision() error {
	_, err := m.f.db.Exec("UPDATE db_info SET value = ? WHERE key=\"revision\"", Revision)
	return err
}

func (m *migrationDb) updateMinor(to int) error {
	_, err := m.f.db.Exec("UPDATE db_info SET value = ? WHERE key=\"minorVersion\"", to)
	return err
}

func (m *migrationDb) updateMajor() error {
	_, err := m.f.db.Exec("UPDATE db_info SET value = ? WHERE key=\"majorVersion\"", MajorVersion)
	return err
}

func (m *migrationDb) migrate3(meta *DbMetadata) error {
	switch meta.MinorVersion {
	case 0:
		// Changes how databases metadata is stored - This was  before migration reworks.
		return fmt.Errorf("cannot migrate from 3.0rX")
	case 1:
		// Cannot use '\' anymore, directories seperators must be '/'
		fmt.Printf("* Migrating from 3.1rX to 3.2rX")
		fmt.Printf("  | Changing all '\\' directory seperators to '/'\n")
		res, err := m.f.db.Exec("UPDATE file SET path=REPLACE(path, \"\\\", \"/\")")
		if err != nil {
			fmt.Printf("  ! Failed: %v\n", err)
			return err
		}
		rows, err := res.RowsAffected()
		if err != nil {
			fmt.Printf("  - Failed to get number of rows affected by change\n")
		} else {
			fmt.Printf("  + Modified %d rows\n", rows)
		}
		err = m.updateMinor(2)
		if err != nil {
			fmt.Printf("  ! Failed to update version: %v\n", err)
			return err
		}
		fmt.Printf("+ Done\n")
		fallthrough
	case 2:
		// Latest
	default:
		return fmt.Errorf("unsupported version, max version is %s", FormatVersion(MajorVersion, MinorVersion, Revision))
	}
	// Revisions can be updated freely.
	return m.updateRevision()
}

func (m *migrationDb) MigrateToLatest() error {
	meta, err := m.f.GetMetadata()
	if err != nil {
		return fmt.Errorf("failed to get version info: %v", err)
	}
	if meta.MajorVersion == MajorVersion && meta.MinorVersion == MinorVersion && meta.RevisionVersion == Revision {
		// Already up to date.
		return nil
	}
	switch meta.MajorVersion {
	case 1, 2:
		return fmt.Errorf("version %d (%s) databases cannot be migrated", meta.MajorVersion, MajorVersionToCodeName(meta.MajorVersion))
	case 3:
		return m.migrate3(meta)
	default:
		return fmt.Errorf("unsupported major version %d", meta.MajorVersion)
	}
}

func DoMigration(d *FileDb) error {
	m := &migrationDb{
		f: d,
	}
	return m.MigrateToLatest()
}
