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
	"bytes"
	"encoding/gob"
	"fmt"

	. "github.com/antonmry/leanmanager/api"
	"github.com/boltdb/bolt"
)

type NotMemberFoundError string

var db *bolt.DB

func (f NotMemberFoundError) Error() string {
	return fmt.Sprintf("Not member found with username %s", string(f))
}

func InitDb(path string) error {
	var err error
	db, err = bolt.Open(path, 0600, nil)
	return err
}

func CloseDb() error {
	return db.Close()
}

func StoreChannel(channelToBeCreated Channel) error {
	return db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(channelToBeCreated.Id))
		if err != nil {
			return fmt.Errorf("dbutils: create bucket: %s", err)
		}
		return nil
	})
}

func StoreMember(member Member) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(member.ChannelId))
		if b == nil {
			return fmt.Errorf("dbutils: bucket %s not created.", member.ChannelId)
		}

		var buf bytes.Buffer
		enc := gob.NewEncoder(&buf)
		enc.Encode(member)

		// Persist bytes to users bucket.
		return b.Put([]byte(member.Id), buf.Bytes())
	})
}

func DeleteMember(channelId, memberId string) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(channelId))
		if b == nil {
			return fmt.Errorf("dbutils: bucket %s not created.", channelId)
		}

		if v := b.Get([]byte(memberId)); v == nil {
			return NotMemberFoundError(memberId)
		}

		return b.Delete([]byte(memberId))
	})
}

func GetMemberByName(channelId, memberName string) (member *Member, err error) {
	err = db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(channelId))
		if b == nil {
			return fmt.Errorf("dbutils: bucket %s not created.", channelId)
		}
		v := b.Get([]byte(memberName))
		if v == nil {
			return NotMemberFoundError(memberName)
		}

		buf := *bytes.NewBuffer(v)
		dec := gob.NewDecoder(&buf)
		dec.Decode(&member)
		return nil
	})

	return
}

func GetMembersByChannel(channelId string, teamMembers *[]Member) error {

	err := db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte(channelId))
		if b == nil {
			return fmt.Errorf("dbutils: bucket %s not created.", channelId)
		}

		c := b.Cursor()
		var member Member

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
