package handler

import "github.com/Wei-Shaw/sub2api/internal/service"

func shouldStopOpenAIOAuth429Failover(
	gateway *service.OpenAIGatewayService,
	account *service.Account,
	failoverErr *service.UpstreamFailoverError,
	switchCount int,
) bool {
	if gateway == nil || failoverErr == nil {
		return false
	}
	gateway.RecordOpenAIOAuth429(account, failoverErr.StatusCode)
	return gateway.ShouldStopOpenAIOAuth429Failover(account, failoverErr.StatusCode, switchCount)
}
