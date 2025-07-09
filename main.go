package main

import (
	"flag"
	"log"
	"strconv"
	
	"gitlab-branch-ruler/internal/config"
	"gitlab-branch-ruler/internal/gitlab"
)

func main() {
	cfgPath := flag.String("config", "config.json", "path to config file")
	flag.Parse()

	cfg, err := config.LoadConfig(*cfgPath)
	if err != nil {
		log.Fatalf("FATAL: load config: %v", err)
	}

	glClient := gitlab.NewClient(cfg.BaseURL, cfg.GitLabToken, cfg.PushAccessLevel, cfg.MergeAccessLevel)

	rootGroup, err := glClient.GetGroup(cfg.RootGroupPath)
	if err != nil {
		log.Fatalf("FATAL: get root group %q: %v", cfg.RootGroupPath, err)
	}

	log.Printf("Processing group: %s (ID=%d)", rootGroup.Name, rootGroup.ID)
	gitlab.ProcessGroup(glClient, strconv.Itoa(rootGroup.ID), rootGroup.Name)
	log.Printf("Finished with group: %s (ID=%d)", rootGroup.Name, rootGroup.ID)
}
