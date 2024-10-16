/*Package api contains base API implementation of unified alerting
 *
 *Generated by: Swagger Codegen (https://github.com/swagger-api/swagger-codegen.git)
 *
 *Do not manually edit these files, please find ngalert/api/swagger-codegen/ for commands on how to generate them.
 */

package api

import (
	"net/http"

	"github.com/grafana/grafana/pkg/api/response"
	"github.com/grafana/grafana/pkg/api/routing"
	"github.com/grafana/grafana/pkg/middleware"
	"github.com/grafana/grafana/pkg/models"
	apimodels "github.com/grafana/grafana/pkg/services/ngalert/api/tooling/definitions"
	"github.com/grafana/grafana/pkg/services/ngalert/metrics"
	"github.com/grafana/grafana/pkg/web"
)

type ProvisioningApiForkingService interface {
	RouteDeleteAlertRule(*models.ReqContext) response.Response
	RouteDeleteContactpoints(*models.ReqContext) response.Response
	RouteInternalDeleteContactpoints(*models.ReqContext) response.Response // LOGZ.IO GRAFANA CHANGE :: DEV-32721 - Internal API to manage contact points
	RouteGetAlertRules(*models.ReqContext) response.Response               // LOGZ.IO GRAFANA CHANGE :: DEV-33330 - API to return all alert rules
	RouteGetAlertRule(*models.ReqContext) response.Response
	RouteGetContactpoints(*models.ReqContext) response.Response
	RouteInternalGetContactpoints(*models.ReqContext) response.Response // LOGZ.IO GRAFANA CHANGE :: DEV-32721 - Internal API to manage contact points
	RouteGetPolicyTree(*models.ReqContext) response.Response
	RoutePostAlertRule(*models.ReqContext) response.Response
	RoutePostContactpoints(*models.ReqContext) response.Response
	RoutePutAlertRule(*models.ReqContext) response.Response
	RoutePutAlertRuleGroup(*models.ReqContext) response.Response
	RoutePutContactpoint(*models.ReqContext) response.Response
	RouteInternalPutContactpoint(*models.ReqContext) response.Response // LOGZ.IO GRAFANA CHANGE :: DEV-32721 - Internal API to manage contact points
	RoutePutPolicyTree(*models.ReqContext) response.Response
	RouteResetPolicyTree(*models.ReqContext) response.Response
}

func (f *ForkedProvisioningApi) RouteDeleteAlertRule(ctx *models.ReqContext) response.Response {
	uIDParam := web.Params(ctx.Req)[":UID"]
	return f.forkRouteDeleteAlertRule(ctx, uIDParam)
}

func (f *ForkedProvisioningApi) RouteDeleteContactpoints(ctx *models.ReqContext) response.Response {
	return f.forkRouteDeleteContactpoints(ctx)
}

// LOGZ.IO GRAFANA CHANGE :: DEV-32721 - Internal API to manage contact points
func (f *ForkedProvisioningApi) RouteInternalDeleteContactpoints(ctx *models.ReqContext) response.Response {
	return f.forkRouteInternalDeleteContactpoints(ctx)
}

// LOGZ.IO GRAFANA CHANGE :: end

// LOGZ.IO GRAFANA CHANGE :: DEV-33330 - API to return all alert rules
func (f *ForkedProvisioningApi) RouteGetAlertRules(ctx *models.ReqContext) response.Response {
	return f.forkRouteGetAlertRules(ctx)
}

// LOGZ.IO GRAFANA CHANGE :: end

func (f *ForkedProvisioningApi) RouteGetAlertRule(ctx *models.ReqContext) response.Response {
	uIDParam := web.Params(ctx.Req)[":UID"]
	return f.forkRouteGetAlertRule(ctx, uIDParam)
}

func (f *ForkedProvisioningApi) RouteGetContactpoints(ctx *models.ReqContext) response.Response {
	return f.forkRouteGetContactpoints(ctx)
}

// LOGZ.IO GRAFANA CHANGE :: DEV-32721 - Internal API to manage contact points
func (f *ForkedProvisioningApi) RouteInternalGetContactpoints(ctx *models.ReqContext) response.Response {
	return f.forkRouteInternalGetContactpoints(ctx)
}

// LOGZ.IO GRAFANA CHANGE :: end

func (f *ForkedProvisioningApi) RouteGetPolicyTree(ctx *models.ReqContext) response.Response {
	return f.forkRouteGetPolicyTree(ctx)
}

func (f *ForkedProvisioningApi) RoutePostAlertRule(ctx *models.ReqContext) response.Response {
	conf := apimodels.AlertRule{}
	if err := web.Bind(ctx.Req, &conf); err != nil {
		return response.Error(http.StatusBadRequest, "bad request data", err)
	}
	return f.forkRoutePostAlertRule(ctx, conf)
}

func (f *ForkedProvisioningApi) RoutePostContactpoints(ctx *models.ReqContext) response.Response {
	conf := apimodels.EmbeddedContactPoint{}
	if err := web.Bind(ctx.Req, &conf); err != nil {
		return response.Error(http.StatusBadRequest, "bad request data", err)
	}
	return f.forkRoutePostContactpoints(ctx, conf)
}

func (f *ForkedProvisioningApi) RoutePutAlertRule(ctx *models.ReqContext) response.Response {
	uIDParam := web.Params(ctx.Req)[":UID"]
	conf := apimodels.AlertRule{}
	if err := web.Bind(ctx.Req, &conf); err != nil {
		return response.Error(http.StatusBadRequest, "bad request data", err)
	}
	return f.forkRoutePutAlertRule(ctx, conf, uIDParam)
}
func (f *ForkedProvisioningApi) RoutePutAlertRuleGroup(ctx *models.ReqContext) response.Response {
	folderUIDParam := web.Params(ctx.Req)[":FolderUID"]
	groupParam := web.Params(ctx.Req)[":Group"]
	conf := apimodels.AlertRuleGroup{}
	if err := web.Bind(ctx.Req, &conf); err != nil {
		return response.Error(http.StatusBadRequest, "bad request data", err)
	}
	return f.forkRoutePutAlertRuleGroup(ctx, conf, folderUIDParam, groupParam)
}

func (f *ForkedProvisioningApi) RoutePutContactpoint(ctx *models.ReqContext) response.Response {
	uIDParam := web.Params(ctx.Req)[":UID"]
	conf := apimodels.EmbeddedContactPoint{}
	if err := web.Bind(ctx.Req, &conf); err != nil {
		return response.Error(http.StatusBadRequest, "bad request data", err)
	}
	return f.forkRoutePutContactpoint(ctx, conf, uIDParam)
}

// LOGZ.IO GRAFANA CHANGE :: DEV-32721 - Internal API to manage contact points
func (f *ForkedProvisioningApi) RouteInternalPutContactpoint(ctx *models.ReqContext) response.Response {
	uIDParam := web.Params(ctx.Req)[":UID"]
	conf := apimodels.EmbeddedContactPoint{}
	if err := web.Bind(ctx.Req, &conf); err != nil {
		return response.Error(http.StatusBadRequest, "bad request data", err)
	}
	return f.forkRouteInternalPutContactpoint(ctx, conf, uIDParam)
}

// LOGZ.IO GRAFANA CHANGE :: end

func (f *ForkedProvisioningApi) RoutePutPolicyTree(ctx *models.ReqContext) response.Response {
	conf := apimodels.Route{}
	if err := web.Bind(ctx.Req, &conf); err != nil {
		return response.Error(http.StatusBadRequest, "bad request data", err)
	}
	return f.forkRoutePutPolicyTree(ctx, conf)
}

func (f *ForkedProvisioningApi) RouteResetPolicyTree(ctx *models.ReqContext) response.Response {
	return f.forkRouteResetPolicyTree(ctx)
}

func (api *API) RegisterProvisioningApiEndpoints(srv ProvisioningApiForkingService, m *metrics.API) {
	api.RouteRegister.Group("", func(group routing.RouteRegister) {
		group.Delete(
			toMacaronPath("/api/v1/provisioning/alert-rules/{UID}"),
			api.authorize(http.MethodDelete, "/api/v1/provisioning/alert-rules/{UID}"),
			metrics.Instrument(
				http.MethodDelete,
				"/api/v1/provisioning/alert-rules/{UID}",
				srv.RouteDeleteAlertRule,
				m,
			),
		)
		group.Delete(
			toMacaronPath("/api/v1/provisioning/contact-points/{ID}"),
			api.authorize(http.MethodDelete, "/api/v1/provisioning/contact-points/{ID}"),
			metrics.Instrument(
				http.MethodDelete,
				"/api/v1/provisioning/contact-points/{ID}",
				srv.RouteDeleteContactpoints,
				m,
			),
		)
		// LOGZ.IO GRAFANA CHANGE :: DEV-32721 - Internal API to manage contact points
		group.Delete(
			toMacaronPath("/api/internal/v1/provisioning/contact-points/{ID}"),
			api.authorize(http.MethodDelete, "/api/internal/v1/provisioning/contact-points/{ID}"),
			metrics.Instrument(
				http.MethodDelete,
				"/api/internal/v1/provisioning/contact-points/{ID}",
				srv.RouteInternalDeleteContactpoints,
				m,
			),
		)
		// LOGZ.IO GRAFANA CHANGE :: end
		// LOGZ.IO GRAFANA CHANGE :: DEV-33330 - API to return all alert rules
		group.Get(
			toMacaronPath("/api/v1/provisioning/alert-rules"),
			api.authorize(http.MethodGet, "/api/v1/provisioning/alert-rules"),
			metrics.Instrument(
				http.MethodGet,
				"/api/v1/provisioning/alert-rules",
				srv.RouteGetAlertRules,
				m,
			),
		)
		// LOGZ.IO GRAFANA CHANGE :: end
		group.Get(
			toMacaronPath("/api/v1/provisioning/alert-rules/{UID}"),
			api.authorize(http.MethodGet, "/api/v1/provisioning/alert-rules/{UID}"),
			metrics.Instrument(
				http.MethodGet,
				"/api/v1/provisioning/alert-rules/{UID}",
				srv.RouteGetAlertRule,
				m,
			),
		)
		group.Get(
			toMacaronPath("/api/v1/provisioning/contact-points"),
			api.authorize(http.MethodGet, "/api/v1/provisioning/contact-points"),
			metrics.Instrument(
				http.MethodGet,
				"/api/v1/provisioning/contact-points",
				srv.RouteGetContactpoints,
				m,
			),
		)
		// LOGZ.IO GRAFANA CHANGE :: DEV-32721 - Internal API to manage contact points
		group.Get(
			toMacaronPath("/api/internal/v1/provisioning/contact-points"),
			api.authorize(http.MethodGet, "/api/internal/v1/provisioning/contact-points"),
			metrics.Instrument(
				http.MethodGet,
				"/api/internal/v1/provisioning/contact-points",
				srv.RouteInternalGetContactpoints,
				m,
			),
		)
		// LOGZ.IO GRAFANA CHANGE :: end
		group.Get(
			toMacaronPath("/api/v1/provisioning/policies"),
			api.authorize(http.MethodGet, "/api/v1/provisioning/policies"),
			metrics.Instrument(
				http.MethodGet,
				"/api/v1/provisioning/policies",
				srv.RouteGetPolicyTree,
				m,
			),
		)
		group.Post(
			toMacaronPath("/api/v1/provisioning/alert-rules"),
			api.authorize(http.MethodPost, "/api/v1/provisioning/alert-rules"),
			metrics.Instrument(
				http.MethodPost,
				"/api/v1/provisioning/alert-rules",
				srv.RoutePostAlertRule,
				m,
			),
		)
		group.Post(
			toMacaronPath("/api/v1/provisioning/contact-points"),
			api.authorize(http.MethodPost, "/api/v1/provisioning/contact-points"),
			metrics.Instrument(
				http.MethodPost,
				"/api/v1/provisioning/contact-points",
				srv.RoutePostContactpoints,
				m,
			),
		)
		group.Put(
			toMacaronPath("/api/v1/provisioning/policies"),
			api.authorize(http.MethodPut, "/api/v1/provisioning/policies"),
			metrics.Instrument(
				http.MethodPut,
				"/api/v1/provisioning/policies",
				srv.RoutePutPolicyTree,
				m,
			),
		)
		group.Put(
			toMacaronPath("/api/v1/provisioning/folder/{FolderUID}/rule-groups/{Group}"),
			api.authorize(http.MethodPut, "/api/v1/provisioning/folder/{FolderUID}/rule-groups/{Group}"),
			metrics.Instrument(
				http.MethodPut,
				"/api/v1/provisioning/folder/{FolderUID}/rule-groups/{Group}",
				srv.RoutePutAlertRuleGroup,
				m,
			),
		)
		group.Put(
			toMacaronPath("/api/v1/provisioning/alert-rules/{UID}"),
			api.authorize(http.MethodPut, "/api/v1/provisioning/alert-rules/{UID}"),
			metrics.Instrument(
				http.MethodPut,
				"/api/v1/provisioning/alert-rules/{UID}",
				srv.RoutePutAlertRule,
				m,
			),
		)
		group.Put(
			toMacaronPath("/api/v1/provisioning/contact-points/{UID}"),
			api.authorize(http.MethodPut, "/api/v1/provisioning/contact-points/{UID}"),
			metrics.Instrument(
				http.MethodPut,
				"/api/v1/provisioning/contact-points/{UID}",
				srv.RoutePutContactpoint,
				m,
			),
		)
		// LOGZ.IO GRAFANA CHANGE :: DEV-32721 - Internal API to manage contact points
		group.Put(
			toMacaronPath("/api/internal/v1/provisioning/contact-points/{UID}"),
			api.authorize(http.MethodPut, "/api/internal/v1/provisioning/contact-points/{UID}"),
			metrics.Instrument(
				http.MethodPut,
				"/api/internal/v1/provisioning/contact-points/{UID}",
				srv.RouteInternalPutContactpoint,
				m,
			),
		)
		// LOGZ.IO GRAFANA CHANGE :: end
		group.Delete(
			toMacaronPath("/api/v1/provisioning/policies"),
			api.authorize(http.MethodDelete, "/api/v1/provisioning/policies"),
			metrics.Instrument(
				http.MethodDelete,
				"/api/v1/provisioning/policies",
				srv.RouteResetPolicyTree,
				m,
			),
		)
	}, middleware.ReqSignedIn)
}
