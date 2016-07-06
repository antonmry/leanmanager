// Package apiserver provides the APIs to build the leanmanager logic
package apiserver

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/antonmry/leanmanager/api"
	"github.com/antonmry/leanmanager/storage"
	"github.com/emicklei/go-restful"
	"github.com/emicklei/go-restful/swagger"
)

// DAO represents the access to the DB, it will be refactored to contain DB access info
type DAO struct {
}

func (dao DAO) register(container *restful.Container) {

	dailyWs := new(restful.WebService)

	dailyWs.
		Path("/dailymeetings").
		Doc("Manage Daily Meetings").
		Consumes(restful.MIME_JSON, restful.MIME_XML).
		Produces(restful.MIME_JSON, restful.MIME_XML)

	dailyWs.Route(dailyWs.POST("").To(dao.createDailyMeeting).
		// docs
		Doc("create a Daily Meeting").
		Operation("createDailyMeeting").
		Reads(api.DailyMeeting{}))

	dailyWs.Route(dailyWs.GET("/{bot-id}/").To(dao.findDailyMeetingsByBot).
		// docs
		Doc("get all Daily Meetings associated to a bot").
		Operation("findDailyMeetingsByBot").
		Param(dailyWs.PathParameter("bot-id", "identifier of the bot").DataType("string")).
		Writes(api.DailyMeeting{}))

	container.Add(dailyWs)

	channelWs := new(restful.WebService)

	channelWs.
		Path("/channels").
		Doc("Manage Channels").
		Consumes(restful.MIME_JSON, restful.MIME_XML).
		Produces(restful.MIME_JSON, restful.MIME_XML)

	channelWs.Route(channelWs.POST("").To(dao.createChannel).
		// docs
		Doc("create a channel").
		Operation("createChannel").
		Reads(api.Channel{}))

	container.Add(channelWs)

	memberWs := new(restful.WebService)

	memberWs.
		Path("/members").
		Doc("Manage Members").
		Consumes(restful.MIME_JSON, restful.MIME_XML).
		Produces(restful.MIME_JSON, restful.MIME_XML)

	memberWs.Route(memberWs.GET("/{channel-id}/{member-id}").To(dao.findMember).
		// docs
		Doc("get a member").
		Operation("findMember").
		Param(memberWs.PathParameter("channel-id", "identifier of the channel").DataType("string")).
		Param(memberWs.PathParameter("member-id", "identifier of the member").DataType("string")).
		Writes(api.Member{}))

	memberWs.Route(memberWs.GET("/{channel-id}/").To(dao.findMembersByChannel).
		// docs
		Doc("get all member in a channel").
		Operation("findMembersByChannel").
		Param(memberWs.PathParameter("channel-id", "identifier of the channel").DataType("string")).
		Writes(api.Member{}))

	memberWs.Route(memberWs.POST("").To(dao.createMember).
		// docs
		Doc("create a member").
		Operation("createMember").
		Reads(api.Member{}))

	memberWs.Route(memberWs.DELETE("/{channel-id}/{member-id}").To(dao.removeMember).
		// docs
		Doc("delete a member").
		Operation("removeMember").
		Param(memberWs.PathParameter("channel-id", "identifier of the channel").DataType("string")).
		Param(memberWs.PathParameter("member-id", "identifier of the member").DataType("string")))

	container.Add(memberWs)
}

func (dao *DAO) createDailyMeeting(request *restful.Request, response *restful.Response) {
	d := new(api.DailyMeeting)
	err := request.ReadEntity(d)
	if err != nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusInternalServerError, err.Error())
		return
	}
	err = storage.StoreDailyMeeting(*d)
	if err != nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusInternalServerError, err.Error())
		return

	}
	response.WriteHeaderAndEntity(http.StatusCreated, d)
	log.Printf("apiserver: daily meeting for channel %s created", d.ChannelID)
}

func (dao DAO) findDailyMeetingsByBot(request *restful.Request, response *restful.Response) {

	botID := request.PathParameter("bot-id")
	var teamDailyMeetings []api.DailyMeeting
	if err := storage.GetDailyMeetingsByBot(botID, &teamDailyMeetings); err != nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusNotFound, "404: Member could not be found.")
		return

	}
	response.WriteEntity(teamDailyMeetings)
	log.Printf("apiserver: %d daily meetings found by bot %s", len(teamDailyMeetings), botID)
}

func (dao *DAO) createChannel(request *restful.Request, response *restful.Response) {
	c := new(api.Channel)
	err := request.ReadEntity(c)
	if err != nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusInternalServerError, err.Error())
		return
	}
	err = storage.StoreChannel(*c)
	if err != nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusInternalServerError, err.Error())
		return

	}
	response.WriteHeaderAndEntity(http.StatusCreated, c)
	log.Printf("apiserver: channel %s created", c.ID)
}

func (dao DAO) findMember(request *restful.Request, response *restful.Response) {

	channelID := request.PathParameter("channel-id")
	memberID := request.PathParameter("member-id")
	m, err := storage.GetMemberByName(channelID, memberID)
	if err != nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusNotFound, "404: Member could not be found.")
		return

	}
	response.WriteEntity(m)
	log.Printf("apiserver: member %s found", m.Name)
}

func (dao DAO) findMembersByChannel(request *restful.Request, response *restful.Response) {

	channelID := request.PathParameter("channel-id")
	var teamMembers []api.Member
	if err := storage.GetMembersByChannel(channelID, &teamMembers); err != nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusNotFound, "404: Member could not be found.")
		return

	}
	response.WriteEntity(teamMembers)
	log.Printf("apiserver: %d members found by channel %s", len(teamMembers), channelID)
}

func (dao *DAO) createMember(request *restful.Request, response *restful.Response) {
	m := new(api.Member)
	err := request.ReadEntity(m)
	if err != nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusInternalServerError, err.Error())
		return
	}
	err = storage.StoreMember(*m)
	if err != nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusInternalServerError, err.Error())
		return

	}
	response.WriteHeaderAndEntity(http.StatusCreated, m)
	log.Printf("apiserver: member %s created", m.Name)
}

func (dao *DAO) removeMember(request *restful.Request, response *restful.Response) {
	memberID := request.PathParameter("member-id")
	channelID := request.PathParameter("channel-id")
	err := storage.DeleteMember(channelID, memberID)
	if err != nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusInternalServerError, err.Error())
		return
	}
	log.Printf("apiserver: member %s deleted", memberID)
}

// LaunchAPIServer is invoked by CLI to initiate the API Server
func LaunchAPIServer(pathDbArg, dbNameArg, hostArg string, portArg int) {

	// Parameters
	portStr := strconv.Itoa(portArg)

	// Database initialization
	err := storage.InitDB(pathDbArg + "/" + dbNameArg + ".db")
	if err != nil {
		log.Fatalf("Error opening the database %s: %s", pathDbArg+"/"+dbNameArg+".db", err)
	}
	defer storage.CloseDB()

	// Only for debug:
	restful.TraceLogger(log.New(os.Stdout, "[restful] ", log.LstdFlags|log.Lshortfile))

	wsContainer := restful.NewContainer()
	dao := DAO{}
	dao.register(wsContainer)

	config := swagger.Config{
		WebServices:    wsContainer.RegisteredWebServices(),
		WebServicesUrl: "http://" + hostArg + ":" + portStr,
		ApiPath:        "/apidocs.json",

		SwaggerPath:     "/apidocs/",
		SwaggerFilePath: "../resources/dist",
	}
	swagger.RegisterSwaggerService(config, wsContainer)

	log.Printf("start listening on %s:%d", hostArg, portArg)
	server := &http.Server{Addr: hostArg + ":" + portStr, Handler: wsContainer}
	log.Fatal(server.ListenAndServe())
}
