package discovery

import (
	"context"
	"testing"
	"time"

	dc "github.com/Bilibili/discovery/conf"
	"github.com/Bilibili/discovery/errors"
	"github.com/Bilibili/discovery/lib/http"
	xtime "github.com/Bilibili/discovery/lib/time"
	"github.com/Bilibili/discovery/model"

	. "github.com/smartystreets/goconvey/convey"
)

var (
	ctx     = context.TODO()
	reg     = defRegisArg()
	rew     = &model.ArgRenew{AppID: "main.arch.test", Hostname: "test1", Zone: "sh001", Env: "pre"}
	cancel  = &model.ArgCancel{AppID: "main.arch.test", Hostname: "test1", Zone: "sh001", Env: "pre"}
	fet     = &model.ArgFetch{AppID: "main.arch.test", Zone: "sh001", Env: "pre", Status: 1}
	set     = &model.ArgSet{AppID: "main.arch.test", Zone: "sh001", Env: "pre"}
	pollArg = newPoll()
)

func newFetchArg() *model.ArgFetchs {
	return &model.ArgFetchs{AppID: []string{"main.arch.test"}, Zone: "sh001", Env: "pre", Status: 1}
}
func newPoll() *model.ArgPolls {
	return &model.ArgPolls{
		Env:             "pre",
		AppID:           []string{"main.arch.test"},
		LatestTimestamp: []int64{0},
	}
}
func defRegisArg() *model.ArgRegister {
	return &model.ArgRegister{
		LatestTimestamp: time.Now().Unix(),
		AppID:           "main.arch.test",
		Hostname:        "test1",
		Color:           "red",
		Zone:            "sh001",
		Env:             "pre",
		Status:          1,
		Metadata:        `{"test":"test","weight":"10"}`,
	}
}

var config = newConfig()

func newConfig() *dc.Config {
	return &dc.Config{
		HTTPClient: &http.ClientConfig{
			Dial:      xtime.Duration(time.Second),
			KeepAlive: xtime.Duration(time.Second * 30),
		},
		HTTPServer: &dc.ServerConfig{Addr: "127.0.0.1:7171"},
		Nodes:      []string{"127.0.0.1:7171"},
	}
}

func TestRegister(t *testing.T) {
	Convey("test Register", t, func() {
		svr := New(config)
		i := model.NewInstance(reg)
		svr.Register(context.TODO(), i, reg)
		ins, err := svr.Fetch(context.TODO(), fet)
		So(err, ShouldBeNil)
		So(len(ins.Instances), ShouldResemble, 1)
		Convey("test metadta", func() {
			for _, is := range ins.Instances {

				So(err, ShouldBeNil)
				for _, i := range is {
					So(i.Metadata["weight"], ShouldEqual, "10")
					So(i.Metadata["test"], ShouldEqual, "test")
				}
			}
		})
	})
}
func TestDiscovery(t *testing.T) {
	Convey("test cancel polls", t, func() {
		svr := New(config)
		reg2 := defRegisArg()
		reg2.Hostname = "test2"
		i1 := model.NewInstance(reg)
		i2 := model.NewInstance(reg2)
		svr.Register(context.TODO(), i1, reg)
		svr.Register(context.TODO(), i2, reg2)
		ch, new, err := svr.Polls(context.TODO(), pollArg)
		So(err, ShouldBeNil)
		So(new, ShouldBeTrue)
		ins := <-ch
		So(len(ins["main.arch.test"].Instances["sh001"]), ShouldEqual, 2)
		pollArg.LatestTimestamp[0] = ins["main.arch.test"].LatestTimestamp
		time.Sleep(time.Second)
		err = svr.Cancel(context.TODO(), cancel)
		So(err, ShouldBeNil)
		ch, new, err = svr.Polls(context.TODO(), pollArg)
		So(err, ShouldBeNil)
		So(new, ShouldBeTrue)
		ins = <-ch
		So(len(ins["main.arch.test"].Instances), ShouldEqual, 1)
	})
}
func TestFetchs(t *testing.T) {
	Convey("test fetch multi appid", t, func() {
		svr := New(config)
		reg2 := defRegisArg()
		reg2.AppID = "appid2"
		i1 := model.NewInstance(reg)
		i2 := model.NewInstance(reg2)
		svr.Register(context.TODO(), i1, reg)
		svr.Register(context.TODO(), i2, reg2)
		fetchs := newFetchArg()
		fetchs.AppID = append(fetchs.AppID, "appid2")
		is, err := svr.Fetchs(ctx, fetchs)
		So(err, ShouldBeNil)
		So(len(is), ShouldResemble, 2)
	})
}
func TestZones(t *testing.T) {
	Convey("test multi zone discovery", t, func() {
		svr := New(config)
		reg2 := defRegisArg()
		reg2.Zone = "sh002"
		i1 := model.NewInstance(reg)
		i2 := model.NewInstance(reg2)
		svr.Register(context.TODO(), i1, reg)
		svr.Register(context.TODO(), i2, reg2)
		ch, new, err := svr.Polls(context.TODO(), newPoll())
		So(err, ShouldBeNil)
		So(new, ShouldBeTrue)
		ins := <-ch
		So(len(ins["main.arch.test"].Instances), ShouldEqual, 2)
		pollArg.Zone = "sh002"
		ch, new, err = svr.Polls(context.TODO(), newPoll())
		So(err, ShouldBeNil)
		So(new, ShouldBeTrue)
		ins = <-ch
		So(len(ins["main.arch.test"].Instances["sh002"]), ShouldEqual, 1)
		Convey("test zone update", func() {
			pollArg.LatestTimestamp = []int64{ins["main.arch.test"].LatestTimestamp}
			pollArg.Zone = ""
			reg3 := defRegisArg()
			reg3.Zone = "sh002"
			reg3.Hostname = "test03"
			i3 := model.NewInstance(reg3)
			svr.Register(context.TODO(), i3, reg3)
			ch, _, err = svr.Polls(context.TODO(), pollArg)
			So(err, ShouldBeNil)
			ins = <-ch
			So(len(ins["main.arch.test"].Instances), ShouldResemble, 2)
			So(len(ins["main.arch.test"].Instances["sh002"]), ShouldResemble, 2)
			So(len(ins["main.arch.test"].Instances["sh001"]), ShouldResemble, 1)
			pollArg.LatestTimestamp = []int64{ins["main.arch.test"].LatestTimestamp}
			_, _, err = svr.Polls(context.TODO(), pollArg)
			So(err, ShouldResemble, errors.NotModified)
		})
	})
}
func TestRenew(t *testing.T) {
	Convey("test Renew", t, func() {
		svr := New(config)
		i := model.NewInstance(reg)
		svr.Register(context.TODO(), i, reg)
		_, err := svr.Renew(context.TODO(), rew)
		So(err, ShouldBeNil)
	})
}

func TestCancel(t *testing.T) {
	Convey("test cancel", t, func() {
		svr := New(config)
		i := model.NewInstance(reg)
		svr.Register(context.TODO(), i, reg)
		err := svr.Cancel(context.TODO(), cancel)
		So(err, ShouldBeNil)
		_, err = svr.Fetch(context.TODO(), fet)
		So(err, ShouldResemble, errors.NothingFound)
	})
}

func TestFetchAll(t *testing.T) {
	Convey("test fetch all", t, func() {
		svr := New(config)
		i := model.NewInstance(reg)
		svr.Register(context.TODO(), i, reg)
		fs := svr.FetchAll(context.TODO())
		So(len(fs), ShouldResemble, 1)
	})
}
