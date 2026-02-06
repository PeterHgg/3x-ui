package service

import (
	"encoding/json"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
)

type ClientService struct{}

// FindInboundsByClientUUID returns all inbounds that contain a client with the given UUID.
// It uses a LIKE query optimization to avoid N+1 issues.
func (s *ClientService) FindInboundsByClientUUID(uuid string) ([]*model.Inbound, error) {
	return s.findInboundsByClientField("id", uuid, "vmess")
}

// FindInboundsByClientPassword returns all inbounds that contain a client with the given Password.
// It uses a LIKE query optimization to avoid N+1 issues.
func (s *ClientService) FindInboundsByClientPassword(password string) ([]*model.Inbound, error) {
	return s.findInboundsByClientField("password", password, "trojan")
}

// SearchInboundAndClient finds a single inbound and its matching client config based on UUID or Password.
// Useful for finding the "primary" user info (traffic, email, expiry) when generating subscriptions.
func (s *ClientService) SearchInboundAndClient(uuid, password string) (*model.Inbound, map[string]interface{}, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound
	query := db.Model(&model.Inbound{}).Preload("ClientStats")

	if uuid != "" {
		query = query.Where("settings LIKE ?", "%"+uuid+"%")
	} else if password != "" {
		query = query.Where("settings LIKE ?", "%"+password+"%")
	} else {
		return nil, nil, nil
	}

	if err := query.Find(&inbounds).Error; err != nil {
		return nil, nil, err
	}

	for _, inbound := range inbounds {
		var settings map[string]interface{}
		if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
			continue
		}

		clients, _ := settings["clients"].([]interface{})
		for _, clientData := range clients {
			client, _ := clientData.(map[string]interface{})

			matched := false
			if uuid != "" && client["id"] == uuid {
				matched = true
			} else if password != "" && client["password"] == password {
				matched = true
			}

			if matched {
				return inbound, client, nil
			}
		}
	}
	return nil, nil, nil
}

func (s *ClientService) findInboundsByClientField(key, value, protocol string) ([]*model.Inbound, error) {
	db := database.GetDB()
	var allInbounds []*model.Inbound
	query := db.Model(&model.Inbound{})

	if protocol != "" {
		query = query.Where("protocol = ?", protocol)
	}

	// Optimization: Filter at DB level first
	query = query.Where("settings LIKE ?", "%"+value+"%")

	if err := query.Find(&allInbounds).Error; err != nil {
		return nil, err
	}

	var result []*model.Inbound
	for _, inbound := range allInbounds {
		var settings map[string]interface{}
		if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
			continue
		}

		if clients, ok := settings["clients"].([]interface{}); ok {
			for _, client := range clients {
				if c, ok := client.(map[string]interface{}); ok {
					if v, ok := c[key].(string); ok && v == value {
						result = append(result, inbound)
						break
					}
				}
			}
		}
	}

	return result, nil
}
