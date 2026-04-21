package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"cloud-agent-monitor/internal/topology/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) SaveServiceNode(ctx context.Context, node *domain.ServiceNode) error {
	if node == nil {
		return domain.ErrInvalidNodeID
	}

	model := r.serviceNodeToModel(node)
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "namespace", "environment", "status", "labels", "request_rate", "error_rate", "latency_p99", "latency_p95", "latency_p50", "pod_count", "ready_pods", "service_type", "maintainer", "team", "updated_at"}),
	}).Create(model).Error
}

func (r *Repository) BatchSaveServiceNodes(ctx context.Context, nodes []*domain.ServiceNode) error {
	if len(nodes) == 0 {
		return nil
	}

	models := make([]*ServiceNode, len(nodes))
	for i, node := range nodes {
		models[i] = r.serviceNodeToModel(node)
	}

	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "namespace", "environment", "status", "labels", "request_rate", "error_rate", "latency_p99", "latency_p95", "latency_p50", "pod_count", "ready_pods", "service_type", "maintainer", "team", "updated_at"}),
	}).CreateInBatches(models, 100).Error
}

func (r *Repository) GetServiceNode(ctx context.Context, id uuid.UUID) (*domain.ServiceNode, error) {
	var model ServiceNode
	err := r.db.WithContext(ctx).Where("id = ?", id.String()).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNodeNotFound
		}
		return nil, fmt.Errorf("failed to get service node: %w", err)
	}
	return r.modelToServiceNode(&model), nil
}

func (r *Repository) GetServiceNodeByName(ctx context.Context, namespace, name string) (*domain.ServiceNode, error) {
	var model ServiceNode
	err := r.db.WithContext(ctx).Where("namespace = ? AND name = ?", namespace, name).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNodeNotFound
		}
		return nil, fmt.Errorf("failed to get service node by name: %w", err)
	}
	return r.modelToServiceNode(&model), nil
}

func (r *Repository) ListServiceNodes(ctx context.Context, query domain.TopologyQuery) ([]*domain.ServiceNode, int64, error) {
	db := r.db.WithContext(ctx).Model(&ServiceNode{})

	if query.HasNamespace() {
		db = db.Where("namespace = ?", query.Namespace)
	}

	if query.ServiceName != "" {
		db = db.Where("name LIKE ?", "%"+query.ServiceName+"%")
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count service nodes: %w", err)
	}

	var models []*ServiceNode
	err := db.Order("updated_at DESC").
		Limit(query.GetLimit()).
		Offset(query.Offset).
		Find(&models).Error
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list service nodes: %w", err)
	}

	nodes := make([]*domain.ServiceNode, len(models))
	for i, model := range models {
		nodes[i] = r.modelToServiceNode(model)
	}

	return nodes, total, nil
}

func (r *Repository) DeleteServiceNode(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Where("id = ?", id.String()).Delete(&ServiceNode{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete service node: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrNodeNotFound
	}
	return nil
}

func (r *Repository) SaveCallEdge(ctx context.Context, edge *domain.CallEdge) error {
	if edge == nil {
		return domain.ErrInvalidEdgeID
	}

	model := r.callEdgeToModel(edge)
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"source_id", "target_id", "edge_type", "is_direct", "confidence", "protocol", "method", "request_rate", "error_rate", "latency_p99", "target_instances", "updated_at"}),
	}).Create(model).Error
}

func (r *Repository) BatchSaveCallEdges(ctx context.Context, edges []*domain.CallEdge) error {
	if len(edges) == 0 {
		return nil
	}

	models := make([]*CallEdge, len(edges))
	for i, edge := range edges {
		models[i] = r.callEdgeToModel(edge)
	}

	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"source_id", "target_id", "edge_type", "is_direct", "confidence", "protocol", "method", "request_rate", "error_rate", "latency_p99", "target_instances", "updated_at"}),
	}).CreateInBatches(models, 100).Error
}

func (r *Repository) GetCallEdge(ctx context.Context, id uuid.UUID) (*domain.CallEdge, error) {
	var model CallEdge
	err := r.db.WithContext(ctx).Where("id = ?", id.String()).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrEdgeNotFound
		}
		return nil, fmt.Errorf("failed to get call edge: %w", err)
	}
	return r.modelToCallEdge(&model), nil
}

func (r *Repository) GetCallEdgeByEndpoints(ctx context.Context, sourceID, targetID uuid.UUID) (*domain.CallEdge, error) {
	var model CallEdge
	err := r.db.WithContext(ctx).
		Where("source_id = ? AND target_id = ?", sourceID.String(), targetID.String()).
		First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrEdgeNotFound
		}
		return nil, fmt.Errorf("failed to get call edge by endpoints: %w", err)
	}
	return r.modelToCallEdge(&model), nil
}

func (r *Repository) ListCallEdges(ctx context.Context, query domain.TopologyQuery) ([]*domain.CallEdge, int64, error) {
	db := r.db.WithContext(ctx).Model(&CallEdge{})

	if query.HasNamespace() {
		db = db.Joins("JOIN topology_service_nodes source ON topology_call_edges.source_id = source.id").
			Where("source.namespace = ?", query.Namespace)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count call edges: %w", err)
	}

	var models []*CallEdge
	err := db.Order("updated_at DESC").
		Limit(query.GetLimit()).
		Offset(query.Offset).
		Find(&models).Error
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list call edges: %w", err)
	}

	edges := make([]*domain.CallEdge, len(models))
	for i, model := range models {
		edges[i] = r.modelToCallEdge(model)
	}

	return edges, total, nil
}

func (r *Repository) ListCallEdgesBySource(ctx context.Context, sourceID uuid.UUID) ([]*domain.CallEdge, error) {
	var models []*CallEdge
	err := r.db.WithContext(ctx).
		Where("source_id = ?", sourceID.String()).
		Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list call edges by source: %w", err)
	}

	edges := make([]*domain.CallEdge, len(models))
	for i, model := range models {
		edges[i] = r.modelToCallEdge(model)
	}
	return edges, nil
}

func (r *Repository) ListCallEdgesByTarget(ctx context.Context, targetID uuid.UUID) ([]*domain.CallEdge, error) {
	var models []*CallEdge
	err := r.db.WithContext(ctx).
		Where("target_id = ?", targetID.String()).
		Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list call edges by target: %w", err)
	}

	edges := make([]*domain.CallEdge, len(models))
	for i, model := range models {
		edges[i] = r.modelToCallEdge(model)
	}
	return edges, nil
}

func (r *Repository) DeleteCallEdge(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Where("id = ?", id.String()).Delete(&CallEdge{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete call edge: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrEdgeNotFound
	}
	return nil
}

func (r *Repository) SaveNetworkNode(ctx context.Context, node *domain.NetworkNode) error {
	if node == nil {
		return domain.ErrInvalidNodeID
	}

	model := r.networkNodeToModel(node)
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "type", "layer", "ip_address", "cidr", "ports", "namespace", "pod_name", "node_name", "zone", "data_center", "connections", "bytes_in", "bytes_out", "packet_loss", "latency", "updated_at"}),
	}).Create(model).Error
}

func (r *Repository) BatchSaveNetworkNodes(ctx context.Context, nodes []*domain.NetworkNode) error {
	if len(nodes) == 0 {
		return nil
	}

	models := make([]*NetworkNode, len(nodes))
	for i, node := range nodes {
		models[i] = r.networkNodeToModel(node)
	}

	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "type", "layer", "ip_address", "cidr", "ports", "namespace", "pod_name", "node_name", "zone", "data_center", "connections", "bytes_in", "bytes_out", "packet_loss", "latency", "updated_at"}),
	}).CreateInBatches(models, 100).Error
}

func (r *Repository) GetNetworkNode(ctx context.Context, id uuid.UUID) (*domain.NetworkNode, error) {
	var model NetworkNode
	err := r.db.WithContext(ctx).Where("id = ?", id.String()).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNodeNotFound
		}
		return nil, fmt.Errorf("failed to get network node: %w", err)
	}
	return r.modelToNetworkNode(&model), nil
}

func (r *Repository) GetNetworkNodeByIP(ctx context.Context, ip string) (*domain.NetworkNode, error) {
	var model NetworkNode
	err := r.db.WithContext(ctx).Where("ip_address = ?", ip).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNodeNotFound
		}
		return nil, fmt.Errorf("failed to get network node by IP: %w", err)
	}
	return r.modelToNetworkNode(&model), nil
}

func (r *Repository) ListNetworkNodes(ctx context.Context, query domain.TopologyQuery) ([]*domain.NetworkNode, int64, error) {
	db := r.db.WithContext(ctx).Model(&NetworkNode{})

	if query.HasNamespace() {
		db = db.Where("namespace = ?", query.Namespace)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count network nodes: %w", err)
	}

	var models []*NetworkNode
	err := db.Order("updated_at DESC").
		Limit(query.GetLimit()).
		Offset(query.Offset).
		Find(&models).Error
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list network nodes: %w", err)
	}

	nodes := make([]*domain.NetworkNode, len(models))
	for i, model := range models {
		nodes[i] = r.modelToNetworkNode(model)
	}

	return nodes, total, nil
}

func (r *Repository) DeleteNetworkNode(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Where("id = ?", id.String()).Delete(&NetworkNode{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete network node: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrNodeNotFound
	}
	return nil
}

func (r *Repository) SaveNetworkEdge(ctx context.Context, edge *domain.NetworkEdge) error {
	if edge == nil {
		return domain.ErrInvalidEdgeID
	}

	model := r.networkEdgeToModel(edge)
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"source_id", "target_id", "source_ip", "target_ip", "source_port", "target_port", "protocol", "bytes_sent", "bytes_received", "packets_sent", "packets_lost", "connection_count", "established", "time_wait", "close_wait", "updated_at"}),
	}).Create(model).Error
}

func (r *Repository) BatchSaveNetworkEdges(ctx context.Context, edges []*domain.NetworkEdge) error {
	if len(edges) == 0 {
		return nil
	}

	models := make([]*NetworkEdge, len(edges))
	for i, edge := range edges {
		models[i] = r.networkEdgeToModel(edge)
	}

	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"source_id", "target_id", "source_ip", "target_ip", "source_port", "target_port", "protocol", "bytes_sent", "bytes_received", "packets_sent", "packets_lost", "connection_count", "established", "time_wait", "close_wait", "updated_at"}),
	}).CreateInBatches(models, 100).Error
}

func (r *Repository) GetNetworkEdge(ctx context.Context, id uuid.UUID) (*domain.NetworkEdge, error) {
	var model NetworkEdge
	err := r.db.WithContext(ctx).Where("id = ?", id.String()).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrEdgeNotFound
		}
		return nil, fmt.Errorf("failed to get network edge: %w", err)
	}
	return r.modelToNetworkEdge(&model), nil
}

func (r *Repository) ListNetworkEdges(ctx context.Context, query domain.TopologyQuery) ([]*domain.NetworkEdge, int64, error) {
	db := r.db.WithContext(ctx).Model(&NetworkEdge{})

	if query.HasNamespace() {
		db = db.Joins("JOIN topology_network_nodes source ON topology_network_edges.source_id = source.id").
			Where("source.namespace = ?", query.Namespace)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count network edges: %w", err)
	}

	var models []*NetworkEdge
	err := db.Order("updated_at DESC").
		Limit(query.GetLimit()).
		Offset(query.Offset).
		Find(&models).Error
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list network edges: %w", err)
	}

	edges := make([]*domain.NetworkEdge, len(models))
	for i, model := range models {
		edges[i] = r.modelToNetworkEdge(model)
	}

	return edges, total, nil
}

func (r *Repository) DeleteNetworkEdge(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Where("id = ?", id.String()).Delete(&NetworkEdge{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete network edge: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrEdgeNotFound
	}
	return nil
}

func (r *Repository) SaveTopologyChange(ctx context.Context, change *domain.TopologyChange) error {
	if change == nil {
		return nil
	}

	model := &TopologyChange{
		ID:          change.ID.String(),
		Timestamp:   change.Timestamp,
		ChangeType:  change.ChangeType,
		EntityType:  change.EntityType,
		EntityID:    change.EntityID.String(),
		EntityName:  change.EntityName,
		Description: change.Description,
	}

	if change.BeforeState != "" {
		model.BeforeState = change.BeforeState
	}
	if change.AfterState != "" {
		model.AfterState = change.AfterState
	}

	return r.db.WithContext(ctx).Create(model).Error
}

func (r *Repository) ListTopologyChanges(ctx context.Context, from, to time.Time, entityType string) ([]*domain.TopologyChange, error) {
	db := r.db.WithContext(ctx).Model(&TopologyChange{})

	if !from.IsZero() {
		db = db.Where("timestamp >= ?", from)
	}
	if !to.IsZero() {
		db = db.Where("timestamp <= ?", to)
	}
	if entityType != "" {
		db = db.Where("entity_type = ?", entityType)
	}

	var models []*TopologyChange
	err := db.Order("timestamp DESC").Limit(1000).Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list topology changes: %w", err)
	}

	changes := make([]*domain.TopologyChange, len(models))
	for i, model := range models {
		changes[i] = &domain.TopologyChange{
			ID:          uuid.MustParse(model.ID),
			Timestamp:   model.Timestamp,
			ChangeType:  model.ChangeType,
			EntityType:  model.EntityType,
			EntityID:    uuid.MustParse(model.EntityID),
			EntityName:  model.EntityName,
			Description: model.Description,
		}
		if model.BeforeState != "" {
			json.Unmarshal([]byte(model.BeforeState), &changes[i].BeforeState)
		}
		if model.AfterState != "" {
			json.Unmarshal([]byte(model.AfterState), &changes[i].AfterState)
		}
	}

	return changes, nil
}

func (r *Repository) SaveTopologySnapshot(ctx context.Context, graphType string, snapshot *domain.ServiceTopology) error {
	if snapshot == nil {
		return nil
	}

	nodesData, _ := json.Marshal(snapshot.Nodes)
	edgesData, _ := json.Marshal(snapshot.Edges)

	model := &TopologySnapshot{
		ID:        snapshot.ID.String(),
		GraphType: graphType,
		Timestamp: snapshot.Timestamp,
		Hash:      snapshot.Hash,
		Nodes:     string(nodesData),
		Edges:     string(edgesData),
	}

	return r.db.WithContext(ctx).Create(model).Error
}

func (r *Repository) GetTopologySnapshot(ctx context.Context, graphType string, timestamp time.Time) (*domain.ServiceTopology, error) {
	var model TopologySnapshot
	err := r.db.WithContext(ctx).
		Where("graph_type = ? AND timestamp <= ?", graphType, timestamp).
		Order("timestamp DESC").
		First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrTopologyEmpty
		}
		return nil, fmt.Errorf("failed to get topology snapshot: %w", err)
	}

	snapshot := &domain.ServiceTopology{
		ID:        uuid.MustParse(model.ID),
		Timestamp: model.Timestamp,
		Hash:      model.Hash,
	}

	if model.Nodes != "" {
		json.Unmarshal([]byte(model.Nodes), &snapshot.Nodes)
	}
	if model.Edges != "" {
		json.Unmarshal([]byte(model.Edges), &snapshot.Edges)
	}

	return snapshot, nil
}

func (r *Repository) CleanupOldData(ctx context.Context, retention time.Duration) error {
	cutoff := time.Now().Add(-retention)

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("timestamp < ?", cutoff).Delete(&TopologyChange{}).Error; err != nil {
			return err
		}
		if err := tx.Where("timestamp < ?", cutoff).Delete(&TopologySnapshot{}).Error; err != nil {
			return err
		}
		return nil
	})
}

func (r *Repository) serviceNodeToModel(node *domain.ServiceNode) *ServiceNode {
	labelsData, _ := json.Marshal(node.Labels)
	return &ServiceNode{
		ID:          node.ID.String(),
		Name:        node.Name,
		Namespace:   node.Namespace,
		Environment: node.Environment,
		Status:      string(node.Status),
		Labels:      string(labelsData),
		RequestRate: node.RequestRate,
		ErrorRate:   node.ErrorRate,
		LatencyP99:  node.LatencyP99,
		LatencyP95:  node.LatencyP95,
		LatencyP50:  node.LatencyP50,
		PodCount:    node.PodCount,
		ReadyPods:   node.ReadyPods,
		ServiceType: node.ServiceType,
		Maintainer:  node.Maintainer,
		Team:        node.Team,
		UpdatedAt:   node.UpdatedAt,
	}
}

func (r *Repository) modelToServiceNode(model *ServiceNode) *domain.ServiceNode {
	node := &domain.ServiceNode{
		ID:          uuid.MustParse(model.ID),
		Name:        model.Name,
		Namespace:   model.Namespace,
		Environment: model.Environment,
		Status:      domain.ServiceStatus(model.Status),
		Labels:      make(map[string]string),
		RequestRate: model.RequestRate,
		ErrorRate:   model.ErrorRate,
		LatencyP99:  model.LatencyP99,
		LatencyP95:  model.LatencyP95,
		LatencyP50:  model.LatencyP50,
		PodCount:    model.PodCount,
		ReadyPods:   model.ReadyPods,
		ServiceType: model.ServiceType,
		Maintainer:  model.Maintainer,
		Team:        model.Team,
		UpdatedAt:   model.UpdatedAt,
	}
	if model.Labels != "" {
		json.Unmarshal([]byte(model.Labels), &node.Labels)
	}
	return node
}

func (r *Repository) callEdgeToModel(edge *domain.CallEdge) *CallEdge {
	targetInstancesData, _ := json.Marshal(edge.TargetInstances)
	return &CallEdge{
		ID:              edge.ID.String(),
		SourceID:        edge.SourceID.String(),
		TargetID:        edge.TargetID.String(),
		EdgeType:        string(edge.EdgeType),
		IsDirect:        edge.IsDirect,
		Confidence:      edge.Confidence,
		Protocol:        edge.Protocol,
		Method:          edge.Method,
		RequestRate:     edge.RequestRate,
		ErrorRate:       edge.ErrorRate,
		LatencyP99:      edge.LatencyP99,
		TargetInstances: string(targetInstancesData),
		UpdatedAt:       edge.UpdatedAt,
	}
}

func (r *Repository) modelToCallEdge(model *CallEdge) *domain.CallEdge {
	edge := &domain.CallEdge{
		ID:          uuid.MustParse(model.ID),
		SourceID:    uuid.MustParse(model.SourceID),
		TargetID:    uuid.MustParse(model.TargetID),
		EdgeType:    domain.EdgeType(model.EdgeType),
		IsDirect:    model.IsDirect,
		Confidence:  model.Confidence,
		Protocol:    model.Protocol,
		Method:      model.Method,
		RequestRate: model.RequestRate,
		ErrorRate:   model.ErrorRate,
		LatencyP99:  model.LatencyP99,
		UpdatedAt:   model.UpdatedAt,
	}
	if model.TargetInstances != "" {
		json.Unmarshal([]byte(model.TargetInstances), &edge.TargetInstances)
	}
	return edge
}

func (r *Repository) networkNodeToModel(node *domain.NetworkNode) *NetworkNode {
	portsData, _ := json.Marshal(node.Ports)
	return &NetworkNode{
		ID:          node.ID.String(),
		Name:        node.Name,
		Type:        node.Type,
		Layer:       string(node.Layer),
		IPAddress:   node.IPAddress,
		CIDR:        node.CIDR,
		Ports:       string(portsData),
		Namespace:   node.Namespace,
		PodName:     node.PodName,
		NodeName:    node.NodeName,
		Zone:        node.Zone,
		DataCenter:  node.DataCenter,
		Connections: node.Connections,
		BytesIn:     node.BytesIn,
		BytesOut:    node.BytesOut,
		PacketLoss:  node.PacketLoss,
		Latency:     node.Latency,
		UpdatedAt:   node.UpdatedAt,
	}
}

func (r *Repository) modelToNetworkNode(model *NetworkNode) *domain.NetworkNode {
	node := &domain.NetworkNode{
		ID:          uuid.MustParse(model.ID),
		Name:        model.Name,
		Type:        model.Type,
		Layer:       domain.NetworkLayer(model.Layer),
		IPAddress:   model.IPAddress,
		CIDR:        model.CIDR,
		Namespace:   model.Namespace,
		PodName:     model.PodName,
		NodeName:    model.NodeName,
		Zone:        model.Zone,
		DataCenter:  model.DataCenter,
		Connections: model.Connections,
		BytesIn:     model.BytesIn,
		BytesOut:    model.BytesOut,
		PacketLoss:  model.PacketLoss,
		Latency:     model.Latency,
		UpdatedAt:   model.UpdatedAt,
	}
	if model.Ports != "" {
		json.Unmarshal([]byte(model.Ports), &node.Ports)
	}
	return node
}

func (r *Repository) networkEdgeToModel(edge *domain.NetworkEdge) *NetworkEdge {
	return &NetworkEdge{
		ID:              edge.ID.String(),
		SourceID:        edge.SourceID.String(),
		TargetID:        edge.TargetID.String(),
		SourceIP:        edge.SourceIP,
		TargetIP:        edge.TargetIP,
		SourcePort:      edge.SourcePort,
		TargetPort:      edge.TargetPort,
		Protocol:        edge.Protocol,
		BytesSent:       edge.BytesSent,
		BytesReceived:   edge.BytesReceived,
		PacketsSent:     edge.PacketsSent,
		PacketsLost:     edge.PacketsLost,
		ConnectionCount: edge.ConnectionCount,
		Established:     edge.Established,
		TimeWait:        edge.TimeWait,
		CloseWait:       edge.CloseWait,
		UpdatedAt:       edge.UpdatedAt,
	}
}

func (r *Repository) modelToNetworkEdge(model *NetworkEdge) *domain.NetworkEdge {
	return &domain.NetworkEdge{
		ID:              uuid.MustParse(model.ID),
		SourceID:        uuid.MustParse(model.SourceID),
		TargetID:        uuid.MustParse(model.TargetID),
		SourceIP:        model.SourceIP,
		TargetIP:        model.TargetIP,
		SourcePort:      model.SourcePort,
		TargetPort:      model.TargetPort,
		Protocol:        model.Protocol,
		BytesSent:       model.BytesSent,
		BytesReceived:   model.BytesReceived,
		PacketsSent:     model.PacketsSent,
		PacketsLost:     model.PacketsLost,
		ConnectionCount: model.ConnectionCount,
		Established:     model.Established,
		TimeWait:        model.TimeWait,
		CloseWait:       model.CloseWait,
		UpdatedAt:       model.UpdatedAt,
	}
}
