package main

import (
	"fmt"
	"github.com/boltdb/bolt"
	"encoding/json"
	"encoding/binary"
)

func createBucket(db *bolt.DB, bucketName string) error {

	return db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		if err != nil {
			return fmt.Errorf("Slackbot: create bucket: %s", err)
		}
		return nil
	})
}

func storeMember(db *bolt.DB, member *memberRecord, bucketName string) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))

		// Generate ID for the user.
		// This returns an error only if the Tx is closed or not writeable.
		// That can't happen in an Update() call so I ignore the error check.
		id, _ := b.NextSequence()
		member.ID = int(id)

		// Marshal user data into bytes.
		buf, err := json.Marshal(member)
		if err != nil {
			return err
		}

		// Persist bytes to users bucket.
		return b.Put(itob(member.ID), buf)
	})
}

func getTeamMembers(db *bolt.DB, bucketName string) ([]memberRecord, error) {

	teamMembers := make([]memberRecord, 0)

	err := db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte(bucketName))

		c := b.Cursor()
		var member memberRecord

		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			if err := json.Unmarshal(k, &member); err != nil {
				return err
			}
			teamMembers = append(teamMembers, member)
		}

		return nil
	})

	return teamMembers, err
}

// itob returns an 8-byte big endian representation of v.
func itob(v int) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}
