package service

import (
	"fmt"
	"net/http"

	"github.com/evergreen-ci/evergreen"
	"github.com/evergreen-ci/evergreen/model"
	"github.com/gorilla/context"
	"github.com/gorilla/mux"
)

type restContextKey int

const RestContext restContextKey = 0

type RouteInfo struct {
	Path    string
	Handler http.HandlerFunc
	Name    string
	Method  string
}

type restAPIService interface {
	WriteJSON(w http.ResponseWriter, status int, data interface{})
	GetSettings() evergreen.Settings
	LoggedError(http.ResponseWriter, *http.Request, int, error)
}

type restAPI struct {
	restAPIService
}

func (ra *restAPI) loadCtx(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		taskId := vars["task_id"]
		buildId := vars["build_id"]
		versionId := vars["version_id"]
		patchId := vars["patch_id"]

		ctx, err := model.LoadContext(taskId, buildId, versionId, patchId, "")
		if err != nil {
			// Some database lookup failed when fetching the data - log it
			ra.LoggedError(w, r, http.StatusInternalServerError, fmt.Errorf("Error loading project context: %v", err))
			return
		}
		if ctx.ProjectRef != nil && ctx.ProjectRef.Private && GetUser(r) == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if ctx.Patch != nil && GetUser(r) == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		context.Set(r, RestContext, ctx)
		next(w, r)
	}
}

// GetRESTContext fetches the context associated with the request..
func GetRESTContext(r *http.Request) (*model.Context, error) {
	if rv := context.Get(r, RequestProjectContext); rv != nil {
		return rv.(*model.Context), nil
	}
	return nil, fmt.Errorf("No context loaded")
}

// MustHaveProjectContext gets the projectContext from the request,
// or panics if it does not exist.
func MustHaveRESTContext(r *http.Request) *model.Context {
	pc, err := GetRESTContext(r)
	if err != nil {
		panic(err)
	}
	return pc
}

func AttachRESTHandler(root *mux.Router, service restAPIService) http.Handler {
	rtr := root.PathPrefix("/rest/v1/").Subrouter()

	// REST routes
	rest := restAPI{service}

	//restRouter := root.PathPrefix("/rest/v1/").Subrouter().StrictSlash(true)
	rtr.HandleFunc("/projects", rest.loadCtx(rest.getProjectIds)).Name("project_list").Methods("GET")
	rtr.HandleFunc("/projects/{project_id}", rest.loadCtx(rest.getProject)).Name("project_info").Methods("GET")
	rtr.HandleFunc("/projects/{project_id}/versions", rest.loadCtx(rest.getRecentVersions)).Name("recent_versions").Methods("GET")
	rtr.HandleFunc("/projects/{project_id}/revisions/{revision}", rest.loadCtx(rest.getVersionInfoViaRevision)).Name("version_info_via_revision").Methods("GET")
	rtr.HandleFunc("/versions/{version_id}", rest.loadCtx(rest.getVersionInfo)).Name("version_info").Methods("GET")
	rtr.HandleFunc("/versions/{version_id}", requireUser(rest.loadCtx(rest.modifyVersionInfo), nil)).Name("").Methods("PATCH")
	rtr.HandleFunc("/versions/{version_id}/status", rest.loadCtx(rest.getVersionStatus)).Name("version_status").Methods("GET")
	rtr.HandleFunc("/builds/{build_id}", rest.loadCtx(rest.getBuildInfo)).Name("build_info").Methods("GET")
	rtr.HandleFunc("/builds/{build_id}/status", rest.loadCtx(rest.getBuildStatus)).Name("build_status").Methods("GET")
	rtr.HandleFunc("/tasks/{task_id}", rest.loadCtx(rest.getTaskInfo)).Name("task_info").Methods("GET")
	rtr.HandleFunc("/tasks/{task_id}/status", rest.loadCtx(rest.getTaskStatus)).Name("task_status").Methods("GET")
	rtr.HandleFunc("/tasks/{task_name}/history", rest.loadCtx(rest.getTaskHistory)).Name("task_history").Methods("GET")
	return root

}
