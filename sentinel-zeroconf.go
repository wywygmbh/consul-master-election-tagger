package main

import (
	"github.com/hashicorp/consul/api"
	"log"
	"os"
	"time"
)

// todo: add as arguments
var queryName string = "sensu-master"
var queryTags []string = []string{"sensu", "master"}
var serviceName string = "redis"
var lockKey string = queryName

func main() {

	client, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		panic(err)
	}

	for i := 0; i < 5; i++ {
		// todo: add lock for lock checking :P
		queryResponse, _, err := getMaster(client)
		if err != nil {
			panic(err)
		}

		if len(queryResponse.Nodes) == 0 {
			lockHeld := consulLock(client, lockKey, 0*time.Second)
			if lockHeld {
				updateTag(client, "master")
				break
			} else {
				time.Sleep(1 * time.Second)
				continue
			}
		} else {
			updateTag(client, "slave")
			break
		}
	}

	os.Exit(0)
}

func updateTag(client *api.Client, tag string) {
	agent := client.Agent()
	services, err := agent.Services()
	if err != nil {
		panic(err)
	}
	service := services[serviceName]

	log.Printf("trying to add tag '%s' to service '%s'", tag, service.Service)

	serviceRegistration := &api.AgentServiceRegistration{
		ID:                service.ID,
		Name:              service.Service,
		Tags:              append(cleanupTagSlice(service.Tags), tag),
		Port:              service.Port,
		Address:           service.Address,
		EnableTagOverride: service.EnableTagOverride,
	}

	err = agent.ServiceRegister(serviceRegistration)
	if err != nil {
		panic(err)
	}

	log.Printf("successfully added tag '%s' to service '%s'", tag, service.Service)
}

func getMaster(client *api.Client) (*api.PreparedQueryExecuteResponse, *api.QueryMeta, error) {
	preparedQuery := client.PreparedQuery()
	preparedQueries, _, err := preparedQuery.List(&api.QueryOptions{})
	if err != nil {
		panic(err)
	}

	var masterQuery api.PreparedQueryDefinition
	for _, query := range preparedQueries {
		if query.Name == queryName {
			log.Printf("found query: %+v", query)
			masterQuery = *query
			break
		}
	}
	if masterQuery.ID == "" {
		log.Println("query not found, creating")

		masterQueryDefinition := api.PreparedQueryDefinition{
			Name: queryName,
			Service: api.ServiceQuery{
				Service:     serviceName,
				OnlyPassing: true,
				Tags:        queryTags,
			},
		}
		newMasterQueryId, _, err := preparedQuery.Create(&masterQueryDefinition, &api.WriteOptions{})
		if err != nil {
			panic(err)
		}
		masterQueryDefinition.ID = newMasterQueryId
		masterQuery = masterQueryDefinition
	}

	return preparedQuery.Execute(masterQuery.ID, &api.QueryOptions{})
}

func consulLock(client *api.Client, key string, lockWaitTime time.Duration) bool {

	//kv := client.KV()
	//session := client.Session()

	lock, err := client.LockOpts(&api.LockOptions{Key: key, LockTryOnce: true, LockWaitTime: lockWaitTime})
	if err != nil {
		panic(err)
	}

	lockChan, err := lock.Lock(nil)
	if err != nil {
		panic(err)
	}
	lockHeld := false
	if lockChan == nil {
		log.Println("lock aquisition failed")
	} else {
		log.Println("got lock")
		lockHeld = true
	}

	return lockHeld
}

// removes the "master" and "slave" from the given slice
func cleanupTagSlice(slice []string) []string {
	var result []string
	for _, v := range slice {
		if v == "master" || v == "slave" {
			continue
		}
		result = append(result, v)
	}

	return result
}
