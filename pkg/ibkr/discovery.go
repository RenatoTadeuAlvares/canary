package ibkr

import (
	"context"
	"errors"
	"strconv"
	"time"
)

// DiscoveryCandidate is a symbolSamples observation. It conveys no admission.
type DiscoveryCandidate struct {
	ConID              int      `json:"conid"`
	Symbol             string   `json:"symbol"`
	SecType            string   `json:"secType"`
	PrimaryExchange    string   `json:"primaryExchange"`
	Currency           string   `json:"currency"`
	DerivativeSecTypes []string `json:"derivativeSecTypes"`
	Description        string   `json:"description,omitempty"`
	IssuerID           string   `json:"issuerId,omitempty"`
}

// DiscoveryDetails contains the full contractData callback fields as public
// reference data, plus validated identity fields. No broker account data is included.
type DiscoveryDetails struct {
	ContractDetailsLite
	ISIN   string   `json:"isin,omitempty"`
	Fields []string `json:"contract_data_fields"`
}

type DiscoveryReceipt struct {
	Generation  uint64               `json:"session_generation"`
	Candidates  []DiscoveryCandidate `json:"candidates"`
	Details     []DiscoveryDetails   `json:"details"`
	RequestedAt time.Time            `json:"requested_at"`
	ReceivedAt  time.Time            `json:"received_at"`
	Complete    bool                 `json:"complete"`
}

var ErrDiscoveryIncomplete = errors.New("contract discovery incomplete or session invalid")

// DiscoverContracts makes only matching-symbol and contract-details requests.
// Every request is bound to the same socket epoch. No quote or order is sent.
func (c *Connector) DiscoverContracts(ctx context.Context, pattern string) (out DiscoveryReceipt, err error) {
	out.RequestedAt = time.Now().UTC()
	defer func() { out.ReceivedAt = time.Now().UTC() }()
	if ctx == nil {
		return out, ErrDiscoveryIncomplete
	}
	binding, ok := c.CaptureSession()
	if !ok || pattern == "" {
		return out, ErrDiscoveryIncomplete
	}
	out.Generation = binding.epoch
	conn := binding.connection
	id, _, err := conn.reserveNextRequestIDForEpoch(binding.epoch)
	if err != nil {
		return out, ErrDiscoveryIncomplete
	}
	defer conn.discardRequestIDReservation(id)
	rows := make(chan []string, 1)
	handler := conn.RegisterHandlerAtEpoch(msgSymbolSamples, func(fields []string, epoch uint64) {
		if epoch != binding.epoch || len(fields) < 2 || fields[1] != strconv.Itoa(id) {
			return
		}
		select {
		case rows <- append([]string(nil), fields...):
		default:
		}
	})
	defer conn.UnregisterHandler(msgSymbolSamples, handler)
	// TWS reqMatchingSymbols (81) has no legacy version field.
	if err = conn.sendMessageWithTypeContextForEpoch(ctx, conn.encodeMsg(81, id, pattern), RequestTypeGeneral, binding.epoch, true); err != nil {
		return out, ErrDiscoveryIncomplete
	}
	select {
	case fields := <-rows:
		out.Candidates, err = parseDiscoveryCandidates(fields, conn.serverVersion)
		if err != nil {
			return out, err
		}
	case <-ctx.Done():
		return out, ErrDiscoveryIncomplete
	}
	for _, candidate := range out.Candidates {
		details, e := c.discoveryDetails(ctx, binding, candidate)
		out.Details = append(out.Details, details...)
		if e != nil {
			return out, e
		}
	}
	if !c.SessionCurrent(binding) {
		return out, ErrDiscoveryIncomplete
	}
	out.Complete = true
	return out, nil
}

func parseDiscoveryCandidates(fields []string, serverVersion int) ([]DiscoveryCandidate, error) {
	cursor := contractDetailsWireCursor{fields: fields, ok: true}
	message, ok := cursor.integer()
	if !ok || message != msgSymbolSamples {
		return nil, ErrDiscoveryIncomplete
	}
	if _, ok = cursor.integer(); !ok {
		return nil, ErrDiscoveryIncomplete
	}
	count, ok := cursor.integer()
	if !ok || count < 0 || count > len(fields) {
		return nil, ErrDiscoveryIncomplete
	}
	result := make([]DiscoveryCandidate, 0, count)
	for range count {
		id, valid := cursor.integer()
		if !valid || id <= 0 {
			return nil, ErrDiscoveryIncomplete
		}
		row := DiscoveryCandidate{ConID: id, Symbol: cursor.string(), SecType: cursor.string(), PrimaryExchange: cursor.string(), Currency: cursor.string()}
		if row.Symbol == "" || row.SecType == "" || row.PrimaryExchange == "" || row.Currency == "" {
			return nil, ErrDiscoveryIncomplete
		}
		derivatives, valid := cursor.integer()
		if !valid || derivatives < 0 || derivatives > len(fields) {
			return nil, ErrDiscoveryIncomplete
		}
		for range derivatives {
			row.DerivativeSecTypes = append(row.DerivativeSecTypes, cursor.string())
		}
		if serverVersion >= 176 {
			row.Description = cursor.string()
			row.IssuerID = cursor.string()
		} // description, issuerId
		result = append(result, row)
	}
	if !cursor.complete() {
		return nil, ErrDiscoveryIncomplete
	}
	return result, nil
}

func (c *Connector) discoveryDetails(ctx context.Context, binding ConnectorSessionBinding, candidate DiscoveryCandidate) ([]DiscoveryDetails, error) {
	conn := binding.connection
	id, _, err := conn.reserveNextRequestIDForEpoch(binding.epoch)
	if err != nil {
		return nil, ErrDiscoveryIncomplete
	}
	defer conn.discardRequestIDReservation(id)
	rows := make(chan []string, 64)
	end := make(chan struct{}, 1)
	overflow := make(chan struct{}, 1)
	handler := conn.RegisterHandlerAtEpoch(msgContractData, func(fields []string, epoch uint64) {
		if epoch != binding.epoch {
			return
		}
		requestIndex := 1
		if conn.serverVersion < minServerVerSizeRules {
			requestIndex = 2
		}
		if len(fields) <= requestIndex || fields[requestIndex] != strconv.Itoa(id) {
			return
		}
		select {
		case rows <- append([]string(nil), fields...):
		default:
			select {
			case overflow <- struct{}{}:
			default:
			}
		}
	})
	terminator := conn.RegisterHandlerAtEpoch(msgContractDataEnd, func(fields []string, epoch uint64) {
		if epoch == binding.epoch && len(fields) >= 3 && fields[2] == strconv.Itoa(id) {
			select {
			case end <- struct{}{}:
			default:
			}
		}
	})
	defer conn.UnregisterHandler(msgContractData, handler)
	defer conn.UnregisterHandler(msgContractDataEnd, terminator)
	contract := Contract{ConID: candidate.ConID, Symbol: candidate.Symbol, SecType: candidate.SecType, Exchange: candidate.PrimaryExchange, PrimaryExch: candidate.PrimaryExchange, Currency: candidate.Currency}
	if err = conn.sendContractDetailsRequestForEpoch(ctx, contract, id, binding.epoch); err != nil {
		return nil, ErrDiscoveryIncomplete
	}
	// Completion is positive contractDetailsEnd; silence never establishes absence.
	select {
	case <-end:
	case <-overflow:
		return nil, ErrDiscoveryIncomplete
	case <-ctx.Done():
		return nil, ErrDiscoveryIncomplete
	}
	if !c.SessionCurrent(binding) {
		return nil, ErrDiscoveryIncomplete
	}
	result := []DiscoveryDetails{}
	for {
		select {
		case <-overflow:
			return nil, ErrDiscoveryIncomplete
		case fields := <-rows:
			row, ok := parseContractDetailsLite(fields, id, conn.serverVersion)
			classification, full := parseContractDetailsClassification(fields, conn.serverVersion)
			if !ok || !full || classification.isinConflict || row.ConID != candidate.ConID {
				return nil, ErrDiscoveryIncomplete
			}
			result = append(result, DiscoveryDetails{ContractDetailsLite: *row, ISIN: classification.isin, Fields: fields})
		default:
			return result, nil
		}
	}
}
