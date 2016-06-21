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

package apiserver

import (
	"log"
	"net/http"

	. "github.com/antonmry/leanmanager/api"
	storage "github.com/antonmry/leanmanager/storage"
	"github.com/emicklei/go-restful"
	"github.com/emicklei/go-restful/swagger"
)

type DAO struct {
}

func (dao DAO) Register(container *restful.Container) {

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
		Reads(Channel{}))

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
		Writes(Member{}))

	memberWs.Route(memberWs.POST("").To(dao.createMember).
		// docs
		Doc("create a member").
		Operation("createMember").
		Reads(Member{}))

	memberWs.Route(memberWs.DELETE("/{channel-id}/{member-id}").To(dao.removeMember).
		// docs
		Doc("delete a member").
		Operation("removeMember").
		Param(memberWs.PathParameter("channel-id", "identifier of the channel").DataType("string")).
		Param(memberWs.PathParameter("member-id", "identifier of the member").DataType("string")))

	container.Add(memberWs)
}

func (dao *DAO) createChannel(request *restful.Request, response *restful.Response) {
	c := new(Channel)
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
}

func (dao DAO) findMember(request *restful.Request, response *restful.Response) {

	channelId := request.PathParameter("channel-id")
	memberId := request.PathParameter("member-id")
	m, err := storage.GetMemberByName(channelId, memberId)
	if err != nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusNotFound, "404: Member could not be found.")
		return

	}
	response.WriteEntity(m)
}

func (dao *DAO) createMember(request *restful.Request, response *restful.Response) {
	m := new(Member)
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
}

func (dao *DAO) removeMember(request *restful.Request, response *restful.Response) {
	memberId := request.PathParameter("member-id")
	channelId := request.PathParameter("channel-id")
	err := storage.DeleteMember(channelId, memberId)
	if err != nil {
		response.AddHeader("Content-Type", "text/plain")
		response.WriteErrorString(http.StatusInternalServerError, err.Error())
		return

	}
}

func LaunchAPIServer(pathdb string) {

	// Database initialization
	err := storage.InitDb(pathdb)
	if err != nil {
		log.Fatalf("Error opening the database %s: %v", pathdb, err)
	}
	defer storage.CloseDb()

	// Only for debug:
	//restful.TraceLogger(log.New(os.Stdout, "[restful] ", log.LstdFlags|log.Lshortfile))

	wsContainer := restful.NewContainer()
	dao := DAO{}
	dao.Register(wsContainer)

	config := swagger.Config{
		WebServices:    wsContainer.RegisteredWebServices(),
		WebServicesUrl: "http://localhost:8080",
		ApiPath:        "/apidocs.json",

		SwaggerPath:     "/apidocs/",
		SwaggerFilePath: "resources/dist",
	}
	swagger.RegisterSwaggerService(config, wsContainer)

	log.Printf("start listening on localhost:8080")
	server := &http.Server{Addr: ":8080", Handler: wsContainer}
	log.Fatal(server.ListenAndServe())
}
