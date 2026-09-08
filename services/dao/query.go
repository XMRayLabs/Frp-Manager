package dao

import "github.com/Sakurame1/frp-manager/services/app"

type Query interface {
	CertQuery
	ClientQuery
	EndpointQuery
	LinkQuery
	NetworkQuery
	ProxyQuery
	ServerQuery
	StatsQuery
	UserQuery
	WireGuardQuery
	WorkerQuery
}

type Mutation interface {
	CertMutation
	ClientMutation
	EndpointMutation
	LinkMutation
	NetworkMutation
	ProxyMutation
	ServerMutation
	StatsMutation
	UserMutation
	WireGuardMutation
	WorkerMutation
	UserGroupMutation
}

// queryImpl/mutationImpl 鏄叿浣撹〃绾у疄鐜扮殑鍩虹缁撴瀯锛堟寔鏈?ctx锛夈€?
type queryImpl struct {
	ctx *app.Context
}

type mutationImpl struct {
	ctx *app.Context
}

// compositeQuery / compositeMutation 缁勫悎鍚勫瓙棰嗗煙瀹炵幇锛屽澶栨毚闇茬粺涓€鍏ュ彛銆?
type compositeQuery struct {
	CertQuery
	ClientQuery
	EndpointQuery
	LinkQuery
	NetworkQuery
	ProxyQuery
	ServerQuery
	StatsQuery
	UserQuery
	WireGuardQuery
	WorkerQuery
}

type compositeMutation struct {
	CertMutation
	ClientMutation
	EndpointMutation
	LinkMutation
	NetworkMutation
	ProxyMutation
	ServerMutation
	StatsMutation
	UserMutation
	WireGuardMutation
	WorkerMutation
	UserGroupMutation
}

func NewQuery(ctx *app.Context) Query {
	base := &queryImpl{ctx: ctx}
	return &compositeQuery{
		CertQuery:      newCertQuery(base),
		ClientQuery:    newClientQuery(base),
		EndpointQuery:  newEndpointQuery(base),
		LinkQuery:      newLinkQuery(base),
		NetworkQuery:   newNetworkQuery(base),
		ProxyQuery:     newProxyQuery(base),
		ServerQuery:    newServerQuery(base),
		StatsQuery:     newStatsQuery(base),
		UserQuery:      newUserQuery(base),
		WireGuardQuery: newWireGuardQuery(base),
		WorkerQuery:    newWorkerQuery(base),
	}
}

func NewMutation(ctx *app.Context) Mutation {
	base := &mutationImpl{ctx: ctx}
	return &compositeMutation{
		CertMutation:      newCertMutation(base),
		ClientMutation:    newClientMutation(base),
		EndpointMutation:  newEndpointMutation(base),
		LinkMutation:      newLinkMutation(base),
		NetworkMutation:   newNetworkMutation(base),
		ProxyMutation:     newProxyMutation(base),
		ServerMutation:    newServerMutation(base),
		StatsMutation:     newStatsMutation(base),
		UserMutation:      newUserMutation(base),
		WireGuardMutation: newWireGuardMutation(base),
		WorkerMutation:    newWorkerMutation(base),
		UserGroupMutation: newUserGroupMutation(base),
	}
}

var (
	_ Query    = (*compositeQuery)(nil)
	_ Mutation = (*compositeMutation)(nil)
)
