package api

import "crypto/subtle"

// enrollmentTokenHeader carries the shared enrollment secret on POST /v1/register.
const enrollmentTokenHeader = "X-Orion-Enrollment-Token"

// enrollmentTokenValid reports whether the caller may register an agent. When
// Core has no ORION_ENROLLMENT_TOKEN configured registration stays open (and is
// only rate limited); when it is configured the presented value must match.
func (s *Server) enrollmentTokenValid(presented string) bool {
	expected := s.cfg.EnrollmentToken
	if expected == "" {
		return true
	}
	return subtle.ConstantTimeCompare([]byte(presented), []byte(expected)) == 1
}
