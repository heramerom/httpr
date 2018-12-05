package httpr

import "sync"

type mapper interface {
	Store(key string, service *Service)
	Load(key string) (s *Service, ok bool)
	Remove(key string)
}

type unsafeMapper map[string]*Service

func (m unsafeMapper) Store(key string, service *Service) {
	m[key] = service
}

func (m unsafeMapper) Load(key string) (s *Service, ok bool) {
	s, ok = m[key]
	return
}

type safeMapper sync.Map

func (m safeMapper) Store(key string, service *Service) {
	sync.Map(m).Store(key, service)
}

func (m safeMapper) Load(key string) (s *Service, ok bool) {
	if ss, ok := sync.Map(m).Load(key); ok {
		s = ss.(*Service)
	}
	return
}

func (m safeMapper) Remove(key string) {
	sync.Map(m).Delete(key)
}

func (m unsafeMapper) Remove(key string) {
	delete(m, key)
}

type Repo struct {
	m mapper
}

func NewRepo() *Repo {
	return &Repo{
		m: make(unsafeMapper),
	}
}

func NewSafeRepo() *Repo {
	return &Repo{
		m: safeMapper(sync.Map{}),
	}
}
