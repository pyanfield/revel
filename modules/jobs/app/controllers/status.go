package controllers

import (
	"github.com/pyanfield/cron"
	"github.com/pyanfield/revel"
	"github.com/pyanfield/revel/modules/jobs/app/jobs"
	"strings"
)

type Jobs struct {
	*revel.Controller
}

func (c Jobs) Status() revel.Result {
	if !strings.HasPrefix(c.Request.RemoteAddr, "127.0.0.1:") {
		return c.Forbidden("%s is not local", c.Request.RemoteAddr)
	}
	entries := jobs.MainCron.Entries()
	return c.Render(entries)
}

func init() {
	revel.TemplateFuncs["castjob"] = func(job cron.Job) *jobs.Job {
		return job.(*jobs.Job)
	}
}
