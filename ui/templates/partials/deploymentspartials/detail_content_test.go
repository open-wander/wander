package deploymentspartials

import (
	"testing"

	"github.com/shoenig/test/must"
)

func TestHasUnpromotedCanaries(t *testing.T) {
	must.False(t, hasUnpromotedCanaries(nil))
	must.False(t, hasUnpromotedCanaries([]TaskGroupStatus{{DesiredCanaries: 1, Promoted: true}}))
	must.True(t, hasUnpromotedCanaries([]TaskGroupStatus{{DesiredCanaries: 1}}))
}
