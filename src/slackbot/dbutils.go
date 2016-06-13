package main

import (
	"fmt"
	"github.com/boltdb/bolt"
	"log"
	"encoding/gob"
	"bytes"
)

type NotMemberFoundError string

func (f NotMemberFoundError) Error() string {
	return fmt.Sprintf("Not member found with username %s", string(f))
}

func createBucket(db *bolt.DB, bucketName string) error {

	return db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		if err != nil {
			return fmt.Errorf("Slackbot: create bucket: %s", err)
		}
		return nil
	})
}

func storeMember(db *bolt.DB, member memberRecord, bucketName string) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))

		var buf bytes.Buffer
		enc := gob.NewEncoder(&buf)
		enc.Encode(member)

		//Delete log
		log.Printf("stored member: %s with key %s", buf.Bytes(), member.name)

		// Persist bytes to users bucket.
		return b.Put([]byte(member.name), buf.Bytes())
	})
}

func deleteMember(db *bolt.DB, member *memberRecord, bucketName string) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if v := b.Get([]byte(member.name)); v == nil {
			return NotMemberFoundError(member.name)
		}

		return b.Delete([]byte(member.name))
	})
}

func getTeamMembers(db *bolt.DB, bucketName string, teamMembers []memberRecord) (error) {

	err := db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte(bucketName))
		c := b.Cursor()
		var member memberRecord

		for k, v := c.First(); k != nil; k, v = c.Next() {

			buf := *bytes.NewBuffer(v)
			dec := gob.NewDecoder(&buf)
			dec.Decode(&member)

			log.Printf("retrieved member: %s with key %s", member, k)
			teamMembers = append(teamMembers, member)
		}

		return nil
	})

	return err
}

