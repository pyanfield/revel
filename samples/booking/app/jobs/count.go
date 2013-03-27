package jobs

import (
	"fmt"
	"github.com/pyanfield/revel"
	"github.com/pyanfield/revel/modules/jobs/app/jobs"
)

// Periodically count the bookings in the database.
type BookingCounter struct{}

func (c BookingCounter) Run() {
	// TODO: Actually run the query.
	fmt.Println("There are N bookings.")
}

func init() {
	revel.OnAppStart(func() {
		jobs.Schedule("@every 1m", BookingCounter{})
	})
}
