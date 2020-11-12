package utils

import (
	"errors"
	"fmt"
	"log"
	"time"
)

const TimeLayout = "2006-01-02 15:04:05"

func ParseStartEndTime(start, end string) (startTime, endTime time.Time, err error) {
	secondsEastOfUTC := int((8 * time.Hour).Seconds())
	beijing := time.FixedZone("Beijing Time", secondsEastOfUTC)

	startTime, err = time.ParseInLocation(TimeLayout, start, beijing)
	if err != nil {
		log.Printf("error start format: %s", start)
		return
	}
	endTime, err = time.ParseInLocation(TimeLayout, end, beijing)
	if err != nil {
		log.Printf("error end format: %s", end)
		return
	}
	if !startTime.Before(endTime) {
		err = errors.New(fmt.Sprintf("start time(%s) must before end time(%s)", startTime.String(), endTime.String()))
		log.Println(err)
		return
	}
	return
}

func ParseBeijingTime(str string) (time.Time, error) {
	secondsEastOfUTC := int((8 * time.Hour).Seconds())
	beijing := time.FixedZone("Beijing Time", secondsEastOfUTC)
	return time.ParseInLocation(TimeLayout, str, beijing)
}
