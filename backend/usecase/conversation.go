package usecase

import (
	"context"
	"fmt"
	"regexp"

	"github.com/samber/lo"

	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/repo/cache"
	"github.com/chaitin/panda-wiki/repo/ipdb"
	"github.com/chaitin/panda-wiki/repo/pg"
)

type ConversationUsecase struct {
	repo         *pg.ConversationRepository
	nodeRepo     *pg.NodeRepository
	geoCacheRepo *cache.GeoRepo
	logger       *log.Logger
	ipRepo       *ipdb.IPAddressRepo
}

func NewConversationUsecase(
	repo *pg.ConversationRepository,
	nodeRepo *pg.NodeRepository,
	geoCacheRepo *cache.GeoRepo,
	logger *log.Logger,
	ipRepo *ipdb.IPAddressRepo,
) *ConversationUsecase {
	return &ConversationUsecase{
		repo:         repo,
		nodeRepo:     nodeRepo,
		geoCacheRepo: geoCacheRepo,
		ipRepo:       ipRepo,
		logger:       logger.WithModule("usecase.conversation"),
	}
}

func (u *ConversationUsecase) CreateChatConversationMessage(ctx context.Context, kbID string, conversation *domain.ConversationMessage) error {
	references := extractReferencesBlock(conversation.ID, conversation.AppID, conversation.Content)
	return u.repo.CreateConversationMessage(ctx, conversation, references)
}

func (u *ConversationUsecase) GetConversationList(ctx context.Context, request *domain.ConversationListReq) (*domain.PaginatedResult[[]*domain.ConversationListItem], error) {
	conversations, total, err := u.repo.GetConversationList(ctx, request)
	if err != nil {
		return nil, err
	}
	// get ip address
	ipAddressMap := make(map[string]*domain.IPAddress)
	lo.Map(conversations, func(conversation *domain.ConversationListItem, _ int) *domain.ConversationListItem {
		if _, ok := ipAddressMap[conversation.RemoteIP]; !ok {
			ipAddress, err := u.ipRepo.GetIPAddress(ctx, conversation.RemoteIP)
			if err != nil {
				u.logger.Error("get ip address failed", log.Error(err), log.String("ip", conversation.RemoteIP))
				return conversation
			}
			ipAddressMap[conversation.RemoteIP] = ipAddress
			conversation.IPAddress = ipAddress
		} else {
			conversation.IPAddress = ipAddressMap[conversation.RemoteIP]
		}
		return conversation
	})
	return domain.NewPaginatedResult(conversations, total), nil
}

func (u *ConversationUsecase) GetConversationDetail(ctx context.Context, conversationID string) (*domain.ConversationDetailResp, error) {
	conversation, err := u.repo.GetConversationDetail(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	// get ip address
	ipAddress, err := u.ipRepo.GetIPAddress(ctx, conversation.RemoteIP)
	if err != nil {
		u.logger.Error("get ip address failed", log.Error(err), log.String("ip", conversation.RemoteIP))
	} else {
		conversation.IPAddress = ipAddress
	}
	// get messages
	messages, err := u.repo.GetConversationMessagesByID(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	conversation.Messages = messages
	// get references
	references, err := u.repo.GetConversationReferences(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	conversation.References = references
	return conversation, nil
}

func extractReferencesBlock(conversationID, appID, text string) []*domain.ConversationReference {
	// match whole reference block
	reBlock := regexp.MustCompile(`(?ms)((?:>|\\u003e)\s*\[\d+\]\.\s*\[.*?\]\(.*?\)\s*\n?)+$`)
	// find the last match index
	lastIndex := -1
	allMatches := reBlock.FindAllStringIndex(text, -1)
	if len(allMatches) > 0 {
		lastIndex = allMatches[len(allMatches)-1][0]
	}

	if lastIndex == -1 {
		return nil
	}

	// extract all references in the last reference block
	block := text[lastIndex:]
	reLine := regexp.MustCompile(`(?m)^(?:>|\\u003e)\s*\[(\d+)\]\.\s*\[(.*?)\]\((.*?)\)`)
	matches := reLine.FindAllStringSubmatch(block, -1)

	refs := make([]*domain.ConversationReference, 0)
	for _, match := range matches {
		if len(match) == 4 {
			refs = append(refs, &domain.ConversationReference{
				Name: match[2],
				URL:  match[3],

				ConversationID: conversationID,
				AppID:          appID,
			})
		}
	}
	return refs
}

func (u *ConversationUsecase) ValidateConversationNonce(ctx context.Context, conversationID, nonce string) error {
	return u.repo.ValidateConversationNonce(ctx, conversationID, nonce)
}

func (u *ConversationUsecase) CreateConversation(ctx context.Context, conversation *domain.Conversation) error {
	if err := u.repo.CreateConversation(ctx, conversation); err != nil {
		return err
	}
	remoteIP := conversation.RemoteIP
	ipAddress, err := u.ipRepo.GetIPAddress(ctx, remoteIP)
	if err != nil {
		u.logger.Warn("get ip address failed", log.Error(err), log.String("ip", remoteIP), log.String("conversation_id", conversation.ID))
	} else {
		location := fmt.Sprintf("%s|%s|%s", ipAddress.Country, ipAddress.Province, ipAddress.City)
		if err := u.geoCacheRepo.SetGeo(ctx, conversation.KBID, location); err != nil {
			u.logger.Warn("set geo cache failed", log.Error(err), log.String("conversation_id", conversation.ID), log.String("ip", remoteIP))
		}
	}
	return nil
}
