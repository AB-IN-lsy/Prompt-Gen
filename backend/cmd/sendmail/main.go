/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-10 19:11:27
 * @FilePath: \electron-go-app\backend\cmd\sendmail\main.go
 * @LastEditTime: 2025-10-10 19:11:31
 */
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"electron-go-app/backend/internal/config"
	domain "electron-go-app/backend/internal/domain/user"
	"electron-go-app/backend/internal/infra/email"
)

func main() {
	to := flag.String("to", "", "recipient email address")
	name := flag.String("name", "", "display name for the recipient")
	flag.Parse()

	if strings.TrimSpace(*to) == "" {
		log.Fatal("missing -to recipient email")
	}

	config.LoadEnvFiles()

	aliyunCfg, enabled, err := email.LoadAliyunConfigFromEnv()
	if err != nil {
		log.Fatalf("load aliyun config failed: %v", err)
	}
	if !enabled {
		log.Fatal("aliyun config not fully set; check ALIYUN_DM_* variables")
	}

	sender, err := email.NewAliyunSender(aliyunCfg)
	if err != nil {
		log.Fatalf("init aliyun sender failed: %v", err)
	}

	user := &domain.User{
		Email:    strings.TrimSpace(*to),
		Username: strings.TrimSpace(*name),
	}

	token := fmt.Sprintf("test-%d", rand.New(rand.NewSource(time.Now().UnixNano())).Int63())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := sender.SendVerification(ctx, user, token); err != nil {
		log.Fatalf("send verification failed: %v", err)
	}

	log.Printf("verification mail dispatched to %s; token=%s", user.Email, token)
}
