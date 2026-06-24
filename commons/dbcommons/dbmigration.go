package dbcommons

import (
	"bufio"
	"embed"
	"fmt"
	"sort"
	"strings"

	"github.com/juggleim/jugglemate-server/commons/tools"
)

//go:embed sqls/*
var sqlFs embed.FS

const (
	JChatAiDbVersionKey = "jmatedb_version"
)

func Upgrade() {
	// upgrade jchat db
	var currVersion int64 = 0
	dao := GlobalConfDao{}
	conf, err := dao.FindByKey(JChatAiDbVersionKey)
	if err == nil && conf != nil {
		ver, err := tools.String2Int64(conf.ConfValue)
		if err == nil && ver > 0 {
			currVersion = ver
		}
	}
	fmt.Println("[JChatAiDbMigration]current version:", currVersion)
	sqlFiles, err := sqlFs.ReadDir("sqls")
	if err == nil {
		neededVers := []int64{}
		for _, sqlFile := range sqlFiles {
			fileName := sqlFile.Name()
			if len(fileName) == 12 {
				fileName = fileName[:8]
			}
			ver, err := tools.String2Int64(fileName)
			if err == nil && ver > 0 {
				neededVers = append(neededVers, ver)
			}
		}
		//sort
		sort.Slice(neededVers, func(i, j int) bool {
			return neededVers[i] < neededVers[j]
		})
		for _, ver := range neededVers {
			if ver > currVersion {
				sqlFileName := fmt.Sprintf("sqls/%d.sql", ver)
				fmt.Println("[DbMigration]start to execute sql file:", sqlFileName)
				err := executeSqlFile(sqlFileName)
				if err == nil {
					fmt.Println("[DbMigration]execute sql file success:", sqlFileName)
					dao.Upsert(GlobalConfDao{
						ConfKey:   JChatAiDbVersionKey,
						ConfValue: fmt.Sprintf("%d", ver),
					})
				}
			}
		}
	}
}

func executeSqlFile(fileName string) error {
	sqlFile, err := sqlFs.Open(fileName)
	if err != nil {
		fmt.Println("[DbMigration_Err]Read sql file err:", err, "file_name:", fileName)
		return err
	}
	defer sqlFile.Close()

	scanner := bufio.NewScanner(sqlFile)
	var queryBuilder strings.Builder
	execQuery := func(query string) error {
		if query == "" {
			return nil
		}
		if err := GetDb().Exec(query).Error; err != nil {
			fmt.Println("[DbMigration_Err]Execute sql error:", err, query)
			return err
		}
		return nil
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}
		queryBuilder.WriteString(line)
		queryBuilder.WriteByte(' ')
		if strings.HasSuffix(line, ";") {
			query := strings.TrimSpace(queryBuilder.String())
			if err := execQuery(query); err != nil {
				return err
			}
			queryBuilder.Reset()
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Println("[DbMigration_Err]Scan sql file err:", err, "file_name:", fileName)
		return err
	}
	if query := strings.TrimSpace(queryBuilder.String()); query != "" {
		if err := execQuery(query); err != nil {
			return err
		}
	}
	return nil
}
