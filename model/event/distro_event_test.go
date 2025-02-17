package event

import (
	"testing"
	"time"

	"github.com/evergreen-ci/birch"
	"github.com/evergreen-ci/evergreen/db"
	. "github.com/smartystreets/goconvey/convey"
)

func TestLoggingDistroEvents(t *testing.T) {
	Convey("When logging distro events, ", t, func() {

		So(db.Clear(EventCollection), ShouldBeNil)

		Convey("logged events should be stored and queryable in sorted order", func() {
			distroId := "distro_id"
			userId := "user_id"
			// simulate ProviderSettingsMap from DistroData
			data := birch.NewDocument().Set(birch.EC.String("ami", "ami-123456")).ExportMap()
			// log some events, sleeping in between to make sure the times are different
			LogDistroAdded(distroId, userId, nil)
			time.Sleep(1 * time.Millisecond)
			LogDistroModified(distroId, userId, data)
			time.Sleep(1 * time.Millisecond)
			LogDistroRemoved(distroId, userId, nil)
			time.Sleep(1 * time.Millisecond)

			// fetch all the events from the database, make sure they are
			// persisted correctly

			eventsForDistro, err := FindLatestPrimaryDistroEvents(distroId, 10)
			So(err, ShouldBeNil)

			event := eventsForDistro[2]
			So(event.EventType, ShouldEqual, EventDistroAdded)
			So(event.ResourceId, ShouldEqual, distroId)

			eventData, ok := event.Data.(*DistroEventData)
			So(ok, ShouldBeTrue)
			So(event.ResourceType, ShouldEqual, ResourceTypeDistro)
			So(eventData.UserId, ShouldEqual, userId)
			So(eventData.Data, ShouldBeNil)

			event = eventsForDistro[1]
			So(event.EventType, ShouldEqual, EventDistroModified)
			So(event.ResourceId, ShouldEqual, distroId)

			eventData, ok = event.Data.(*DistroEventData)
			So(ok, ShouldBeTrue)
			So(event.ResourceType, ShouldEqual, ResourceTypeDistro)
			So(eventData.UserId, ShouldEqual, userId)

			doc := birch.NewDocument()
			doc.ExtendInterface(eventData.Data)
			ami, ok := doc.Lookup("ami").StringValueOK()
			So(ok, ShouldBeTrue)
			So(ami, ShouldEqual, "ami-123456")

			event = eventsForDistro[0]
			So(event.EventType, ShouldEqual, EventDistroRemoved)
			So(event.ResourceId, ShouldEqual, distroId)

			eventData, ok = event.Data.(*DistroEventData)
			So(ok, ShouldBeTrue)
			So(event.ResourceType, ShouldEqual, ResourceTypeDistro)
			So(eventData.UserId, ShouldEqual, userId)
			So(eventData.Data, ShouldBeNil)
		})
	})
}
