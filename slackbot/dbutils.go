// Copyright © 2016 leanmanager
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package slackbot

import (
	"fmt"
	"github.com/boltdb/bolt"
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
			return fmt.Errorf("dbutils: create bucket: %s", err)
		}
		return nil
	})
}

func storeMember(db *bolt.DB, member memberRecord, bucketName string) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return fmt.Errorf("dbutils: bucket %s not created.", channelId)
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
			return fmt.Errorf("dbutils: bucket %s not created.", channelId)
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
			return fmt.Errorf("dbutils: bucket %s not created.", channelId)
		}

		c := b.Cursor()
		var member memberRecord

		for k, v := c.First(); k != nil; k, v = c.Next() {

			buf := *bytes.NewBuffer(v)
			dec := gob.NewDecoder(&buf)
			dec.Decode(&member)
			*teamMembers = append(*teamMembers, member)
		}

		return nil
	})

	return err
}

