// Copyright https://github.com/Foxboron/terraform-provider-openwrt/graphs/contributors 2025, 2026
// SPDX-License-Identifier: MPL-2.0

package fs_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/foxboron/terraform-provider-openwrt/internal/api/luci"
	"github.com/foxboron/terraform-provider-openwrt/internal/testutil"

	"github.com/foxboron/terraform-provider-openwrt/mocks"
	tfjson "github.com/hashicorp/terraform-json"
	"go.uber.org/mock/gomock"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const configFileContent = "config test 'probe'\n\toption value 'hello'\n"

// fakeRouter keeps /etc/config/smoketest in memory and fails uci commit and
// revert once the file is gone, like a real router does.
type fakeRouter struct {
	files   map[string][]byte
	commits int
}

func newFakeRouter(t *testing.T, ctrl *gomock.Controller, rpcFactory *mocks.MockRPCFactory) *fakeRouter {
	r := &fakeRouter{files: map[string][]byte{}}
	theRPC := mocks.NewMockRPC(ctrl)

	theRPC.
		EXPECT().
		Writefile(gomock.Any(), "/etc/config/smoketest", gomock.Any()).
		DoAndReturn(func(_ context.Context, path string, data []byte) error {
			t.Logf("Writefile method called")
			r.files[path] = data
			return nil
		}).
		AnyTimes()

	theRPC.
		EXPECT().
		ReadFile(gomock.Any(), "/etc/config/smoketest").
		DoAndReturn(func(_ context.Context, path string) ([]byte, error) {
			t.Logf("ReadFile method called")
			data, ok := r.files[path]
			if !ok {
				return nil, errors.New("file not found")
			}
			return data, nil
		}).
		AnyTimes()

	theRPC.
		EXPECT().
		RemoveFile(gomock.Any(), "/etc/config/smoketest").
		DoAndReturn(func(_ context.Context, path string) error {
			t.Logf("RemoveFile method called")
			delete(r.files, path)
			return nil
		}).
		AnyTimes()

	theRPC.
		EXPECT().
		CommitOrRevert(gomock.Any(), "smoketest").
		DoAndReturn(func(_ context.Context, _ ...any) error {
			t.Logf("CommitOrRevert method called")
			r.commits++
			if _, ok := r.files["/etc/config/smoketest"]; !ok {
				return errors.Join(luci.ErrUCICommit, luci.ErrUCIRevert)
			}
			return nil
		}).
		AnyTimes()

	rpcFactory.
		EXPECT().
		Get(gomock.Any(), "http://test.lan:8080", "root", "test", gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _, _ string, _ luci.Timeouts) (luci.RPC, error) {
			t.Logf("Get method called")
			return theRPC, nil
		}).
		AnyTimes()

	return r
}

func (r *fakeRouter) checkRemoved(_ *terraform.State) error {
	if _, ok := r.files["/etc/config/smoketest"]; ok {
		return errors.New("/etc/config/smoketest still exists")
	}
	return nil
}

func TestAccConfigFile_CreateAndDestroy(t *testing.T) {
	os.Setenv("TF_ACC", "1")    //nolint:errcheck
	defer os.Unsetenv("TF_ACC") //nolint:errcheck

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	rpcFactory := mocks.NewMockRPCFactory(ctrl)
	router := newFakeRouter(t, ctrl, rpcFactory)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.TestAccFactories(rpcFactory),
		CheckDestroy:             router.checkRemoved,
		Steps: []resource.TestStep{
			{
				Config: `
				provider "openwrt" {
					user     = "root"
					password = "test"
					remote   = "http://test.lan:8080"
				}

				resource "openwrt_configfile" "smoke" {
					name    = "smoketest"
					content = "config test 'probe'\n\toption value 'hello'\n"
				}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("openwrt_configfile.smoke", "content", configFileContent),
					resource.TestCheckResourceAttr("openwrt_configfile.smoke", "commit", "true"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						&testutil.ActionPlanChecker{
							Addr:   "openwrt_configfile.smoke",
							Action: tfjson.ActionCreate,
						},
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func TestAccConfigFile_DestroyWithoutCommit(t *testing.T) {
	os.Setenv("TF_ACC", "1")    //nolint:errcheck
	defer os.Unsetenv("TF_ACC") //nolint:errcheck

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	rpcFactory := mocks.NewMockRPCFactory(ctrl)
	router := newFakeRouter(t, ctrl, rpcFactory)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testutil.TestAccFactories(rpcFactory),
		CheckDestroy: func(s *terraform.State) error {
			if router.commits != 0 {
				return errors.New("CommitOrRevert called although commit = false")
			}
			return router.checkRemoved(s)
		},
		Steps: []resource.TestStep{
			{
				Config: `
				provider "openwrt" {
					user     = "root"
					password = "test"
					remote   = "http://test.lan:8080"
				}

				resource "openwrt_configfile" "smoke" {
					name    = "smoketest"
					content = "config test 'probe'\n\toption value 'hello'\n"
					commit  = false
				}`,
			},
		},
	})
}
