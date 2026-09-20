package rpc

import (
	"context"

	"connectrpc.com/connect"

	"noteboard/internal/entity"
	pb "noteboard/internal/gen/noteboard/v1"
)

type boardServer struct{ svc BoardService }

func (s *boardServer) GetBoard(_ context.Context, _ *connect.Request[pb.GetBoardRequest]) (*connect.Response[pb.GetBoardResponse], error) {
	cols, err := s.svc.Board()
	if err != nil {
		return nil, errorOf(err)
	}
	out := &pb.GetBoardResponse{Columns: make([]*pb.Column, 0, len(cols))}
	for _, c := range cols {
		out.Columns = append(out.Columns, columnPB(c))
	}
	return connect.NewResponse(out), nil
}

func (s *boardServer) CreateColumn(_ context.Context, req *connect.Request[pb.CreateColumnRequest]) (*connect.Response[pb.CreateColumnResponse], error) {
	c, err := s.svc.CreateColumn(req.Msg.GetTitle())
	if err != nil {
		return nil, errorOf(err)
	}
	return connect.NewResponse(&pb.CreateColumnResponse{Column: columnPB(c)}), nil
}

func (s *boardServer) RenameColumn(_ context.Context, req *connect.Request[pb.RenameColumnRequest]) (*connect.Response[pb.RenameColumnResponse], error) {
	if err := s.svc.RenameColumn(req.Msg.GetId(), req.Msg.GetTitle()); err != nil {
		return nil, errorOf(err)
	}
	return connect.NewResponse(&pb.RenameColumnResponse{}), nil
}

func (s *boardServer) DeleteColumn(_ context.Context, req *connect.Request[pb.DeleteColumnRequest]) (*connect.Response[pb.DeleteColumnResponse], error) {
	if err := s.svc.DeleteColumn(req.Msg.GetId()); err != nil {
		return nil, errorOf(err)
	}
	return connect.NewResponse(&pb.DeleteColumnResponse{}), nil
}

func (s *boardServer) CreateCard(_ context.Context, req *connect.Request[pb.CreateCardRequest]) (*connect.Response[pb.CreateCardResponse], error) {
	m := req.Msg
	c, err := s.svc.CreateCard(m.GetColumnId(), m.GetTitle(), m.GetBody(), m.NoteId)
	if err != nil {
		return nil, errorOf(err)
	}
	return connect.NewResponse(&pb.CreateCardResponse{Card: cardPB(c)}), nil
}

func (s *boardServer) AttachNote(_ context.Context, req *connect.Request[pb.AttachNoteRequest]) (*connect.Response[pb.AttachNoteResponse], error) {
	c, err := s.svc.AttachNote(req.Msg.GetNoteId(), req.Msg.GetColumnId())
	if err != nil {
		return nil, errorOf(err)
	}
	return connect.NewResponse(&pb.AttachNoteResponse{Card: cardPB(c)}), nil
}

func (s *boardServer) UpdateCard(_ context.Context, req *connect.Request[pb.UpdateCardRequest]) (*connect.Response[pb.UpdateCardResponse], error) {
	m := req.Msg
	err := s.svc.UpdateCard(m.GetId(), entity.CardPatch{Title: m.Title, Body: m.Body, NoteID: m.NoteId})
	if err != nil {
		return nil, errorOf(err)
	}
	return connect.NewResponse(&pb.UpdateCardResponse{}), nil
}

func (s *boardServer) DeleteCard(_ context.Context, req *connect.Request[pb.DeleteCardRequest]) (*connect.Response[pb.DeleteCardResponse], error) {
	if err := s.svc.DeleteCard(req.Msg.GetId()); err != nil {
		return nil, errorOf(err)
	}
	return connect.NewResponse(&pb.DeleteCardResponse{}), nil
}

func (s *boardServer) Reorder(_ context.Context, req *connect.Request[pb.ReorderRequest]) (*connect.Response[pb.ReorderResponse], error) {
	orders := make([]entity.ColumnOrder, 0, len(req.Msg.GetColumns()))
	for _, c := range req.Msg.GetColumns() {
		orders = append(orders, entity.ColumnOrder{ColumnID: c.GetColumnId(), CardIDs: c.GetCardIds()})
	}
	if err := s.svc.Reorder(orders); err != nil {
		return nil, errorOf(err)
	}
	return connect.NewResponse(&pb.ReorderResponse{}), nil
}
