package supervisor_test

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/starkandwayne/shield/supervisor"
	"net/http"

	// sql drivers
	_ "github.com/mattn/go-sqlite3"
)

var _ = Describe("HTTP API /v1/schedule", func() {
	var API http.Handler
	var resyncChan chan int

	BeforeEach(func() {
		data, err := Database(
			`INSERT INTO schedules (uuid, name, summary, timespec) VALUES
				("51e69607-eb48-4679-afd2-bc3b4c92e691",
				 "Weekly Backups",
				 "A schedule for weekly bosh-blobs, during normal maintenance windows",
				 "sundays at 3:15am")`,

			`INSERT INTO schedules (uuid, name, summary, timespec) VALUES
				("647bc775-b07b-4f87-bb67-d84cccac34a7",
				 "Daily Backups",
				 "Use for daily (11-something-at-night) bosh-blobs",
				 "daily at 11:24pm")`,

			`INSERT INTO jobs (uuid, schedule_uuid) VALUES ("abc-def", "51e69607-eb48-4679-afd2-bc3b4c92e691")`,
		)
		Ω(err).ShouldNot(HaveOccurred())

		resyncChan = make(chan int, 1)
		API = ScheduleAPI{
			Data:       data,
			ResyncChan: resyncChan,
		}
	})

	AfterEach(func() {
		close(resyncChan)
		resyncChan = nil
	})

	It("should retrieve all schedules", func() {
		res := GET(API, "/v1/schedules")
		Ω(res.Body.String()).Should(MatchJSON(`[
				{
					"uuid"    : "647bc775-b07b-4f87-bb67-d84cccac34a7",
					"name"    : "Daily Backups",
					"summary" : "Use for daily (11-something-at-night) bosh-blobs",
					"when"    : "daily at 11:24pm"
				},
				{
					"uuid"    : "51e69607-eb48-4679-afd2-bc3b4c92e691",
					"name"    : "Weekly Backups",
					"summary" : "A schedule for weekly bosh-blobs, during normal maintenance windows",
					"when"    : "sundays at 3:15am"
				}
			]`))
		Ω(res.Code).Should(Equal(200))
	})

	It("should retrieve only unused schedules for ?unused=t", func() {
		res := GET(API, "/v1/schedules?unused=t")
		Ω(res.Body.String()).Should(MatchJSON(`[
				{
					"uuid"    : "647bc775-b07b-4f87-bb67-d84cccac34a7",
					"name"    : "Daily Backups",
					"summary" : "Use for daily (11-something-at-night) bosh-blobs",
					"when"    : "daily at 11:24pm"
				}
			]`))
		Ω(res.Code).Should(Equal(200))
	})

	It("should retrieve only used schedules for ?unused=f", func() {
		res := GET(API, "/v1/schedules?unused=f")
		Ω(res.Body.String()).Should(MatchJSON(`[
				{
					"uuid"    : "51e69607-eb48-4679-afd2-bc3b4c92e691",
					"name"    : "Weekly Backups",
					"summary" : "A schedule for weekly bosh-blobs, during normal maintenance windows",
					"when"    : "sundays at 3:15am"
				}
			]`))
		Ω(res.Code).Should(Equal(200))
	})

	It("can create new schedules", func() {
		res := POST(API, "/v1/schedules", WithJSON(`{
			"name"    : "My New Schedule",
			"summary" : "A new schedule",
			"when"    : "daily 2pm"
		}`))
		Ω(res.Code).Should(Equal(200))
		Ω(res.Body.String()).Should(MatchRegexp(`{"ok":"created","uuid":"[a-z0-9-]+"}`))
		Eventually(resyncChan).Should(Receive())
	})

	It("requires the `name' and `when' keys in POST'ed data", func() {
		res := POST(API, "/v1/schedules", "{}")
		Ω(res.Code).Should(Equal(400))
	})

	It("can update existing schedules", func() {
		res := PUT(API, "/v1/schedule/647bc775-b07b-4f87-bb67-d84cccac34a7", WithJSON(`{
			"name"    : "Daily Backup Schedule",
			"summary" : "UPDATED!",
			"when"    : "daily at 2:05pm"
		}`))
		Ω(res.Code).Should(Equal(200))
		Ω(res.Body.String()).Should(MatchJSON(`{"ok":"updated"}`))
		Eventually(resyncChan).Should(Receive())

		res = GET(API, "/v1/schedules")
		Ω(res.Body.String()).Should(MatchJSON(`[
				{
					"uuid"    : "647bc775-b07b-4f87-bb67-d84cccac34a7",
					"name"    : "Daily Backup Schedule",
					"summary" : "UPDATED!",
					"when"    : "daily at 2:05pm"
				},
				{
					"uuid"    : "51e69607-eb48-4679-afd2-bc3b4c92e691",
					"name"    : "Weekly Backups",
					"summary" : "A schedule for weekly bosh-blobs, during normal maintenance windows",
					"when"    : "sundays at 3:15am"
				}
			]`))
		Ω(res.Code).Should(Equal(200))
	})

	It("requires the `name' field to update an existing schedule", func() {
		res := PUT(API, "/v1/schedule/647bc775-b07b-4f87-bb67-d84cccac34a7", WithJSON(`{
			"summary" : "UPDATED!",
			"when"    : "daily at 2:05pm"
		}`))
		Ω(res.Code).Should(Equal(400))
	})

	It("requires the `summary' field to update an existing schedule", func() {
		res := PUT(API, "/v1/schedule/647bc775-b07b-4f87-bb67-d84cccac34a7", WithJSON(`{
			"name"    : "Daily Backup Schedule",
			"when"    : "daily at 2:05pm"
		}`))
		Ω(res.Code).Should(Equal(400))
	})

	It("requires the `when' field to update an existing schedule", func() {
		res := PUT(API, "/v1/schedule/647bc775-b07b-4f87-bb67-d84cccac34a7", WithJSON(`{
			"name"    : "Daily Backup Schedule",
			"summary" : "UPDATED!"
		}`))
		Ω(res.Code).Should(Equal(400))
	})

	It("can delete unused schedules", func() {
		res := DELETE(API, "/v1/schedule/647bc775-b07b-4f87-bb67-d84cccac34a7")
		Ω(res.Code).Should(Equal(200))
		Ω(res.Body.String()).Should(MatchJSON(`{"ok":"deleted"}`))
		Eventually(resyncChan).Should(Receive())

		res = GET(API, "/v1/schedules")
		Ω(res.Body.String()).Should(MatchJSON(`[
				{
					"uuid"    : "51e69607-eb48-4679-afd2-bc3b4c92e691",
					"name"    : "Weekly Backups",
					"summary" : "A schedule for weekly bosh-blobs, during normal maintenance windows",
					"when"    : "sundays at 3:15am"
				}
			]`))
		Ω(res.Code).Should(Equal(200))
	})

	It("refuses to delete a schedule that is in use", func() {
		res := DELETE(API, "/v1/schedule/51e69607-eb48-4679-afd2-bc3b4c92e691")
		Ω(res.Code).Should(Equal(403))
		Ω(res.Body.String()).Should(Equal(""))
	})

	It("ignores other HTTP methods", func() {
		for _, method := range []string{"PUT", "DELETE", "PATCH", "OPTIONS", "TRACE"} {
			NotImplemented(API, method, "/v1/schedules", nil)
		}

		for _, method := range []string{"GET", "HEAD", "POST", "PATCH", "OPTIONS", "TRACE"} {
			NotImplemented(API, method, "/v1/schedules/sub/requests", nil)
			NotImplemented(API, method, "/v1/schedule/sub/requests", nil)
			NotImplemented(API, method, "/v1/schedule/5981f34c-ef58-4e3b-a91e-428480c68100", nil)
		}
	})

	It("ignores malformed UUIDs", func() {
		for _, id := range []string{"malformed-uuid-01234", "", "(abcdef-01234-56-789)"} {
			NotImplemented(API, "GET", fmt.Sprintf("/v1/schedule/%s", id), nil)
			NotImplemented(API, "PUT", fmt.Sprintf("/v1/schedule/%s", id), nil)
		}
	})
})
