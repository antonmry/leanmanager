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
		if b == nil {
			return fmt.Errorf("Slackbot: bucket %s not created.", channelId)
		}

		var buf bytes.Buffer
		enc := gob.NewEncoder(&buf)
		enc.Encode(member)

		// Persist bytes to users bucket.
		return b.Put([]byte(member.Name), buf.Bytes())
	})
}

func deleteMember(db *bolt.DB, member *memberRecord, bucketName string) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return fmt.Errorf("Slackbot: bucket %s not created.", channelId)
		}

		if v := b.Get([]byte(member.Name)); v == nil {
			return NotMemberFoundError(member.Name)
		}

		return b.Delete([]byte(member.Name))
	})
}

func getTeamMembers(db *bolt.DB, bucketName string, teamMembers *[]memberRecord) (error) {

	err := db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return fmt.Errorf("Slackbot: bucket %s not created.", channelId)
		}

		c := b.Cursor()
		var member memberRecord

		for k, v := c.First(); k != nil; k, v = c.Next() {

			buf := *bytes.NewBuffer(v)
			dec := gob.NewDecoder(&buf)
			dec.Decode(&member)

			log.Printf("retrieved member: %s with key %s", member, k)
			*teamMembers = append(*teamMembers, member)
		}

		return nil
	})

	return err
}

