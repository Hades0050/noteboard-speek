package rpc

import (
	"context"

	"connectrpc.com/connect"

	"noteboard/internal/entity"
	pb "noteboard/internal/gen/noteboard/v1"
)

// notesServer — тонкая обёртка: распаковать запрос, вызвать сценарий,
// упаковать ответ. Логики здесь нет и быть не должно.
type notesServer struct{ svc NoteService }

func (s *notesServer) ListNotes(_ context.Context, req *connect.Request[pb.ListNotesRequest]) (*connect.Response[pb.ListNotesResponse], error) {
	list, err := s.svc.List(req.Msg.GetQuery())
	if err != nil {
		return nil, errorOf(err)
	}
	out := &pb.ListNotesResponse{Notes: make([]*pb.Note, 0, len(list))}
	for _, n := range list {
		out.Notes = append(out.Notes, notePB(n))
	}
	return connect.NewResponse(out), nil
}

func (s *notesServer) GetNote(_ context.Context, req *connect.Request[pb.GetNoteRequest]) (*connect.Response[pb.GetNoteResponse], error) {
	n, err := s.svc.Get(req.Msg.GetId())
	if err != nil {
		return nil, errorOf(err)
	}
	return connect.NewResponse(&pb.GetNoteResponse{Note: notePB(n)}), nil
}

func (s *notesServer) CreateNote(_ context.Context, req *connect.Request[pb.CreateNoteRequest]) (*connect.Response[pb.CreateNoteResponse], error) {
	n, err := s.svc.Create(req.Msg.GetTitle(), req.Msg.GetBody())
	if err != nil {
		return nil, errorOf(err)
	}
	return connect.NewResponse(&pb.CreateNoteResponse{Note: notePB(n)}), nil
}

func (s *notesServer) UpdateNote(_ context.Context, req *connect.Request[pb.UpdateNoteRequest]) (*connect.Response[pb.UpdateNoteResponse], error) {
	m := req.Msg
	n, err := s.svc.Update(m.GetId(), entity.NotePatch{
		Title:      m.Title,
		Body:       m.Body,
		Transcript: m.Transcript,
		Pinned:     m.Pinned,
	})
	if err != nil {
		return nil, errorOf(err)
	}
	return connect.NewResponse(&pb.UpdateNoteResponse{Note: notePB(n)}), nil
}

func (s *notesServer) DeleteNote(_ context.Context, req *connect.Request[pb.DeleteNoteRequest]) (*connect.Response[pb.DeleteNoteResponse], error) {
	if err := s.svc.Delete(req.Msg.GetId()); err != nil {
		return nil, errorOf(err)
	}
	return connect.NewResponse(&pb.DeleteNoteResponse{}), nil
}

func (s *notesServer) Retranscribe(_ context.Context, req *connect.Request[pb.RetranscribeRequest]) (*connect.Response[pb.RetranscribeResponse], error) {
	n, err := s.svc.Retranscribe(req.Msg.GetId())
	if err != nil {
		return nil, errorOf(err)
	}
	return connect.NewResponse(&pb.RetranscribeResponse{Note: notePB(n)}), nil
}
