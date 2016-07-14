// Package storage contains the logic to persist data in the DB
package storage

import (
	"bytes"
	"encoding/gob"
	"fmt"

	"github.com/antonmry/leanmanager/api"
	"github.com/boltdb/bolt"
)

// NotMemberFoundError is returned when member isn't stored in the database
type NotMemberFoundError string

var db *bolt.DB

func (f NotMemberFoundError) Error() string {
	return fmt.Sprintf("Not member found with username %s", string(f))
}

// InitDB initializes the database, creating or opening the file
func InitDB(path string) error {
	var err error
	db, err = bolt.Open(path, 0600, nil)
	if err != nil {
		return err
	}

	err = db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte("dailymeetings")); err != nil {
			return fmt.Errorf("dbutils: create bucket: %s", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	return db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte("predefinedreplies")); err != nil {
			return fmt.Errorf("dbutils: create bucket: %s", err)
		}
		return nil
	})
}

// CloseDB terminate the DB Session in a properly way
func CloseDB() error {
	return db.Close()
}

// StoreChannel create a bucket by channel where data can be stored
func StoreChannel(channelToBeCreated api.Channel) error {
	return db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(channelToBeCreated.ID))
		if err != nil {
			return fmt.Errorf("dbutils: create bucket: %s", err)
		}
		return nil
	})
}

// StoreMember persists a member inside a bucket identifying the channel
func StoreMember(member api.Member) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(member.ChannelID))
		if b == nil {
			return fmt.Errorf("dbutils: bucket %s not created", member.ChannelID)
		}

		var buf bytes.Buffer
		enc := gob.NewEncoder(&buf)
		enc.Encode(member)

		// Persist bytes to users bucket.
		return b.Put([]byte(member.ID), buf.Bytes())
	})
}

// DeleteMember deletes a member from a bucket which identifies the channel
func DeleteMember(channelID, memberID string) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(channelID))
		if b == nil {
			return fmt.Errorf("dbutils: bucket %s not created", channelID)
		}

		if v := b.Get([]byte(memberID)); v == nil {
			return NotMemberFoundError(memberID)
		}

		return b.Delete([]byte(memberID))
	})
}

// GetMemberByName returns member which is identified by a string, the name
func GetMemberByName(channelID, memberName string) (member *api.Member, err error) {
	err = db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(channelID))
		if b == nil {
			return fmt.Errorf("dbutils: bucket %s not created", channelID)
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

// GetMembersByChannel returns all members stored in a bucket
func GetMembersByChannel(channelID string, teamMembers *[]api.Member) error {

	err := db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte(channelID))
		if b == nil {
			return fmt.Errorf("dbutils: bucket %s not created", channelID)
		}

		c := b.Cursor()
		var member api.Member

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

// StoreDailyMeeting persists the members and configuration of a Daily Meeting
func StoreDailyMeeting(daily api.DailyMeeting) error {
	// TODO: persist by TeamID or BotID
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("dailymeetings"))
		if b == nil {
			return fmt.Errorf("dbutils: bucket dailymeetings not created")
		}

		var buf bytes.Buffer
		enc := gob.NewEncoder(&buf)
		enc.Encode(daily)

		// Persist bytes to daily meetings bucket.
		return b.Put([]byte(daily.ChannelID), buf.Bytes())
	})
}

// GetDailyMeetingsByBot returns all the daily meeting configuration associated to a bot
func GetDailyMeetingsByBot(botID string, teamDailyMeetings *[]api.DailyMeeting) error {

	// TODO: we are not filtering by botID
	err := db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte("dailymeetings"))
		if b == nil {
			return fmt.Errorf("dbutils: bucket dailymeetings not created")
		}

		d := b.Cursor()
		var daily api.DailyMeeting

		for k, v := d.First(); k != nil; k, v = d.Next() {

			buf := *bytes.NewBuffer(v)
			dec := gob.NewDecoder(&buf)
			dec.Decode(&daily)
			*teamDailyMeetings = append(*teamDailyMeetings, daily)
		}

		return nil
	})

	return err
}

// StorePredefinedReply saves a predefined reply used to reply to Daily Meeting answers
func StorePredefinedReply(reply api.PredefinedDailyReply) error {
	// TODO: persist by TeamID or BotID
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("predefinedreplies"))
		if b == nil {
			return fmt.Errorf("dbutils: bucket predefinedreplies not created")
		}

		var buf bytes.Buffer
		enc := gob.NewEncoder(&buf)
		enc.Encode(reply)

		// Persist bytes to daily meetings bucket.
		return b.Put([]byte(reply.ChannelID), buf.Bytes())
	})
}

// GetPredefinedReplies returns all replies associated to answers in a Daily Meeting
func GetPredefinedReplies(channelID string, replies *[]api.PredefinedDailyReply) error {

	err := db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte("predefinedreplies"))
		if b == nil {
			return fmt.Errorf("dbutils: bucket predefinedreplies not created")
		}

		d := b.Cursor()
		var reply api.PredefinedDailyReply

		for k, v := d.First(); k != nil; k, v = d.Next() {

			buf := *bytes.NewBuffer(v)
			dec := gob.NewDecoder(&buf)
			dec.Decode(&reply)
			if reply.ChannelID == channelID {
				*replies = append(*replies, reply)
			}
		}

		return nil
	})

	return err
}

// DeletePredefinedRepliesByChannel deletes all replies associated to answers in a Daily Meeting channel
func DeletePredefinedRepliesByChannel(channelID string) error {
	err := db.Update(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte("predefinedreplies"))
		if b == nil {
			return fmt.Errorf("dbutils: bucket predefinedreplies not created")
		}

		d := b.Cursor()
		var reply api.PredefinedDailyReply

		for k, v := d.First(); k != nil; k, v = d.Next() {

			buf := *bytes.NewBuffer(v)
			dec := gob.NewDecoder(&buf)
			dec.Decode(&reply)
			if reply.ChannelID == channelID {
				if err := b.Delete(k); err != nil {
					return err
				}
			}
		}

		return nil
	})

	return err
}
