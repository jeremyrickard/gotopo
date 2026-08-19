package gotopo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// GetAccountData refreshes and returns the signed-in account data.
func (c *Client) GetAccountData(ctx context.Context) (AccountData, error) {
	accountID := c.credentials.AccountID
	if accountID == "" {
		return AccountData{}, fmt.Errorf("gotopo: account ID is required")
	}
	path := "/api/v1/acct/" + accountID + "/since/0"
	var payload any
	if !c.hosted {
		path = "/sideload/account/" + accountID + ".json"
		payload = map[string]string{"json": `{"full":true}`}
	}
	var data AccountData
	if err := c.do(ctx, requestSpec{method: http.MethodGet, path: path, payload: payload}, &data); err != nil {
		return AccountData{}, err
	}
	c.mu.Lock()
	c.accountData = &data
	c.mu.Unlock()
	return cloneAccountData(data)
}

// GetMapLists returns map lists for matching accounts. An empty account title
// selects personal accounts; a non-empty title selects exactly one group.
func (c *Client) GetMapLists(ctx context.Context, accountTitle string, includeBookmarks, refresh bool) ([]AccountMapList, error) {
	data, err := c.cachedAccountData(ctx, refresh)
	if err != nil {
		return nil, err
	}
	accounts := make([]Feature, 0)
	for _, account := range data.Accounts {
		isGroup := strings.Contains(account.Properties.String("subscriptionType"), "team")
		if accountTitle == "" {
			if !isGroup {
				accounts = append(accounts, account)
			}
		} else if isGroup && account.Title() == accountTitle {
			accounts = append(accounts, account)
		}
	}
	if accountTitle != "" && len(accounts) == 0 {
		return nil, ErrNotFound
	}
	if accountTitle != "" && len(accounts) > 1 {
		return nil, ErrAmbiguousMatch
	}
	result := make([]AccountMapList, 0, len(accounts))
	for _, account := range accounts {
		folderNames := map[string]string{}
		for _, feature := range data.Features {
			if feature.Class() == "UserFolder" && feature.Properties.String("accountId") == account.ID {
				folderNames[feature.ID] = strings.TrimSpace(feature.Title())
			}
		}
		maps := make([]MapInfo, 0)
		for _, feature := range data.Features {
			if feature.Class() != "CollaborativeMap" || feature.Properties.String("accountId") != account.ID {
				continue
			}
			folderID := feature.Properties.String("folderId")
			info := MapInfo{
				ID: feature.ID, Title: feature.Title(), Updated: int64Number(feature.Properties["updated"]),
				Type: "map", FolderID: folderID, FolderName: folderName(folderNames, folderID),
			}
			info.Locked, _ = feature.Properties["locked"].(bool)
			maps = append(maps, info)
		}
		if includeBookmarks {
			for _, rel := range data.Rels {
				if rel.Class() != "UserAccountMapRel" || rel.Properties.String("accountId") != account.ID {
					continue
				}
				folderID := rel.Properties.String("folderId")
				maps = append(maps, MapInfo{
					ID: rel.Properties.String("mapId"), Title: rel.Title(),
					Updated: int64Number(rel.Properties["mapUpdated"]), Type: "bookmark",
					Permission: bookmarkPermission(int64Number(rel.Properties["type"])),
					FolderID:   folderID, FolderName: folderName(folderNames, folderID),
				})
			}
		}
		sort.SliceStable(maps, func(i, j int) bool { return maps[i].Updated > maps[j].Updated })
		result = append(result, AccountMapList{
			AccountTitle: account.Title(),
			Personal:     !strings.Contains(account.Properties.String("subscriptionType"), "team"),
			Maps:         maps,
		})
	}
	return result, nil
}

// GetAllMapLists returns group map lists and optionally personal map lists.
func (c *Client) GetAllMapLists(ctx context.Context, includePersonal, includeBookmarks, refresh bool) ([]AccountMapList, error) {
	data, err := c.cachedAccountData(ctx, refresh)
	if err != nil {
		return nil, err
	}
	result := make([]AccountMapList, 0)
	if includePersonal {
		personal, err := c.GetMapLists(ctx, "", includeBookmarks, false)
		if err != nil {
			return nil, err
		}
		result = append(result, personal...)
	}
	for _, account := range data.Accounts {
		if !strings.Contains(account.Properties.String("subscriptionType"), "team") {
			continue
		}
		lists, err := c.GetMapLists(ctx, account.Title(), includeBookmarks, false)
		if err != nil {
			return nil, err
		}
		result = append(result, lists...)
	}
	return result, nil
}

func (c *Client) GetMapTitle(ctx context.Context, mapID string, refresh bool) (string, error) {
	data, err := c.cachedAccountData(ctx, refresh)
	if err != nil {
		return "", err
	}
	if mapID == "" {
		mapID = c.MapID()
	}
	if mapID == "" {
		return "", ErrNoMap
	}
	var title string
	for _, feature := range data.Features {
		if strings.EqualFold(feature.ID, mapID) {
			if title != "" {
				return "", ErrAmbiguousMatch
			}
			title = feature.Title()
		}
	}
	if title == "" {
		return "", ErrNotFound
	}
	return title, nil
}

func (c *Client) GetGroupAccountTitles(ctx context.Context, refresh bool) ([]string, error) {
	data, err := c.cachedAccountData(ctx, refresh)
	if err != nil {
		return nil, err
	}
	titles := make([]string, 0)
	for _, account := range data.Accounts {
		if strings.Contains(account.Properties.String("subscriptionType"), "team") {
			titles = append(titles, account.Title())
		}
	}
	sort.Strings(titles)
	return titles, nil
}

func (c *Client) GetAccountsAndFolders(ctx context.Context, refresh bool) ([]AccountFolders, error) {
	data, err := c.cachedAccountData(ctx, refresh)
	if err != nil {
		return nil, err
	}
	all := make([]AccountFolders, 0, len(data.Accounts))
	for _, account := range data.Accounts {
		byParent := make(map[string][]Feature)
		for _, folder := range data.Features {
			if folder.Class() == "UserFolder" && folder.Properties.String("accountId") == account.ID {
				byParent[folder.Properties.String("folderId")] = append(byParent[folder.Properties.String("folderId")], folder)
			}
		}
		paths := make(map[string]string)
		var build func(string, string, map[string]bool) []AccountFolder
		build = func(parent, prefix string, visiting map[string]bool) []AccountFolder {
			folders := append([]Feature(nil), byParent[parent]...)
			sort.Slice(folders, func(i, j int) bool { return folders[i].Title() < folders[j].Title() })
			out := make([]AccountFolder, 0, len(folders))
			for _, folder := range folders {
				if visiting[folder.ID] {
					continue
				}
				title := strings.TrimSpace(folder.Title())
				path := title
				if prefix != "" {
					path = prefix + "/" + title
				}
				next := make(map[string]bool, len(visiting)+1)
				for id, value := range visiting {
					next[id] = value
				}
				next[folder.ID] = true
				paths[path] = folder.ID
				out = append(out, AccountFolder{
					ID: folder.ID, Title: title, Path: path,
					Subfolders: build(folder.ID, path, next),
				})
			}
			return out
		}
		all = append(all, AccountFolders{
			AccountID: account.ID, AccountTitle: strings.TrimSpace(account.Title()),
			Folders: build("", "", map[string]bool{}), PathsAndIDs: paths,
		})
	}
	return all, nil
}

func (c *Client) cachedAccountData(ctx context.Context, refresh bool) (AccountData, error) {
	c.mu.RLock()
	cached := c.accountData
	c.mu.RUnlock()
	if refresh || cached == nil {
		return c.GetAccountData(ctx)
	}
	return cloneAccountData(*cached)
}

func cloneAccountData(data AccountData) (AccountData, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return AccountData{}, fmt.Errorf("gotopo: clone account data: %w", err)
	}
	var out AccountData
	if err := json.Unmarshal(b, &out); err != nil {
		return AccountData{}, fmt.Errorf("gotopo: clone account data: %w", err)
	}
	return out, nil
}

func folderName(names map[string]string, id string) string {
	if id == "" {
		return "<Top Level>"
	}
	if name := names[id]; name != "" {
		return name
	}
	return "<Unknown>"
}

func bookmarkPermission(value int64) string {
	switch value {
	case 10:
		return "read"
	case 16:
		return "update"
	case 20:
		return "write"
	default:
		return "unknown"
	}
}

func int64Number(value any) int64 {
	switch value := value.(type) {
	case float64:
		return int64(value)
	case int64:
		return value
	case int:
		return int64(value)
	default:
		return 0
	}
}
