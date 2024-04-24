package main_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	. "github.com/paketo-buildpacks/occam/matchers"
	"github.com/sclevine/spec"
)

const repoResponseBase string = `[{
"name" : "%s",
"url" : "example-URL",
"owner" : {
		"login" : "%s"
	}
}]`

const closedPRsResponseBase string = `[{
"title" : "pr-1",
"number" : 1,
"merged_at" : "%s",
"created_at" : "some-garbage",
"user" : {
	"login" : "%s"
	},
"_links" : {
	"commits" : {
			"href" : "https://api.server.com/repos/%s/%s/pulls/1/commits"
		}
	}
}]`

const closedPRCommitsResponseBase string = `[{
  "commit": {
    "committer": {
      "name": "%s",
      "email": "noreply@github.com",
      "date": "%s"
    },
    "message": "example-commit-message"
  }
}]`

func TestMergeTimeCalculator(t *testing.T) {
	var Expect = NewWithT(t).Expect

	mergeTimeCalculator, err := gexec.Build("github.com/initializ-buildpacks/github-config/scripts/time-to-merge")
	Expect(err).NotTo(HaveOccurred())

	spec.Run(t, "scripts/time-to-merge", func(t *testing.T, context spec.G, it spec.S) {
		var (
			Expect     = NewWithT(t).Expect
			Eventually = NewWithT(t).Eventually

			mockGithubServer    *httptest.Server
			mockGithubServerURI string

			initializCommunityRepoResponse  string
			initializBuildpacksRepoResponse string

			initializBuildpacksClosedPRsResponse string
			initializCommunityClosedPRsResponse  string

			initializBuildpacksCommitsResponse string
			initializCommunityCommitsResponse  string
		)

		it.Before(func() {
			initializBuildpacksRepoResponse = fmt.Sprintf(repoResponseBase, "example-repo", "initializ-buildpacks")
			initializCommunityRepoResponse = fmt.Sprintf(repoResponseBase, "other-example-repo", "initializ-community")

			initializBuildpacksClosedPRsResponse = fmt.Sprintf(closedPRsResponseBase,
				time.Now().UTC().Format(time.RFC3339),
				"example-contributor",
				"initializ-buildpacks",
				"example-repo")
			initializCommunityClosedPRsResponse = fmt.Sprintf(closedPRsResponseBase,
				time.Now().UTC().Format(time.RFC3339),
				"other-example-contributor",
				"initializ-community",
				"other-example-repo")

			initializBuildpacksCommitsResponse = fmt.Sprintf(closedPRCommitsResponseBase,
				"example-committer",
				time.Now().UTC().Add(-1*time.Hour).Format(time.RFC3339))
			initializCommunityCommitsResponse = fmt.Sprintf(closedPRCommitsResponseBase,
				"other-example-committer",
				time.Now().UTC().Add(-15*time.Minute).Format(time.RFC3339))

			mockGithubServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method == http.MethodHead {
					http.Error(w, "NotFound", http.StatusNotFound)
					return
				}

				switch req.URL.Path {
				case "/orgs/initializ-buildpacks/repos":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, initializBuildpacksRepoResponse)

				case "/orgs/initializ-community/repos":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, initializCommunityRepoResponse)

				case "/repos/initializ-buildpacks/example-repo/pulls":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, initializBuildpacksClosedPRsResponse)

				case "/repos/initializ-community/other-example-repo/pulls":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, initializCommunityClosedPRsResponse)

				case "/repos/initializ-buildpacks/example-repo/pulls/1/commits":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, initializBuildpacksCommitsResponse)

				case "/repos/initializ-community/other-example-repo/pulls/1/commits":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, initializCommunityCommitsResponse)
				default:
					t.Fatal(fmt.Sprintf("unknown path: %s", req.URL.Path))
				}
			}))

		})

		it.After(func() {
			mockGithubServer.Close()
		})

		context("given a valid auth token is provided", func() {
			it.Before(func() {
				os.Setenv("PAT", "some-token")
			})
			it("correctly calculates median merge time of closed PRs from the past 30 days", func() {
				command := exec.Command(mergeTimeCalculator, "--server", mockGithubServer.URL)
				fmt.Println(mockGithubServerURI)
				buffer := gbytes.NewBuffer()
				session, err := gexec.Start(command, buffer, buffer)

				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(0), func() string { return string(buffer.Contents()) })

				out := string(buffer.Contents())

				Expect(out).To(ContainLines(
					`Pull request initializ-buildpacks/example-repo #1 by example-contributor`,
					`took 60.000000 minutes to merge.`,
				))

				Expect(out).To(ContainLines(
					`Pull request initializ-community/other-example-repo #1 by other-example-contributor`,
					`took 15.000000 minutes to merge.`,
				))
			})
		})

		context("given no auth token has been provided", func() {
			it.Before(func() {
				os.Setenv("PAT", "")
			})

			it("exits and says that an auth token is needed", func() {

				command := exec.Command(mergeTimeCalculator)
				fmt.Println(command)
				buffer := gbytes.NewBuffer()
				session, err := gexec.Start(command, buffer, buffer)

				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })

				out := string(buffer.Contents())

				Expect(out).To(ContainLines(`Please set PAT`))
			})
		})

		it.After(func() {
			os.Setenv("PAT", "abcdefg")
		})

		context("given there is an issue getting repos from an org", func() {
			it.Before(func() {
				mockGithubServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					if req.Method == http.MethodHead {
						http.Error(w, "NotFound", http.StatusNotFound)
						return
					}

					switch req.URL.Path {
					case "/orgs/initializ-buildpacks/repos":
						fmt.Fprintf(w, "unknown path: %s\n", req.URL.Path)

					case "/orgs/initializ-community/repos":
						w.WriteHeader(http.StatusOK)
						fmt.Fprintln(w, initializCommunityRepoResponse)

					case "/repos/initializ-buildpacks/example-repo/pulls":
						w.WriteHeader(http.StatusOK)
						fmt.Fprintln(w, initializBuildpacksClosedPRsResponse)

					case "/repos/initializ-community/other-example-repo/pulls":
						w.WriteHeader(http.StatusOK)
						fmt.Fprintln(w, initializCommunityClosedPRsResponse)

					case "/repos/initializ-buildpacks/example-repo/pulls/1/commits":
						w.WriteHeader(http.StatusOK)
						fmt.Fprintln(w, initializBuildpacksCommitsResponse)

					case "/repos/initializ-community/other-example-repo/pulls/2/commits":
						w.WriteHeader(http.StatusOK)
						fmt.Fprintln(w, initializCommunityCommitsResponse)
					default:
						t.Fatal(fmt.Sprintf("unknown path: %s", req.URL.Path))
					}
				}))

				os.Setenv("PAT", "abcdefg")
			})

			it.After(func() {
				mockGithubServer.Close()
			})

			it("exits with the appropriate error", func() {

				command := exec.Command(mergeTimeCalculator, "--server", mockGithubServer.URL)
				fmt.Println(command)
				buffer := gbytes.NewBuffer()
				session, err := gexec.Start(command, buffer, buffer)

				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })

				out := string(buffer.Contents())

				Expect(out).To(ContainSubstring("failed to calculate merge times: failed to get repositories:"))
				Expect(out).NotTo(ContainLines(
					`Pull request initializ-community/other-example-repo #2 by other-example-contributor`,
					`took 15.000000 minutes to merge.`,
				))
			})
		})

		context("given there is an issue getting pull requests from a repo", func() {
			it.Before(func() {
				mockGithubServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					if req.Method == http.MethodHead {
						http.Error(w, "NotFound", http.StatusNotFound)
						return
					}

					switch req.URL.Path {
					case "/orgs/initializ-buildpacks/repos":
						w.WriteHeader(http.StatusOK)
						fmt.Fprintln(w, initializBuildpacksRepoResponse)

					case "/orgs/initializ-community/repos":
						w.WriteHeader(http.StatusOK)
						fmt.Fprintln(w, initializCommunityRepoResponse)

					case "/repos/initializ-buildpacks/example-repo/pulls":
						fmt.Fprintf(w, "unknown path: %s\n", req.URL.Path)

					case "/repos/initializ-community/other-example-repo/pulls":
						w.WriteHeader(http.StatusOK)
						fmt.Fprintln(w, initializCommunityClosedPRsResponse)

					case "/repos/initializ-buildpacks/example-repo/pulls/1/commits":
						w.WriteHeader(http.StatusOK)
						fmt.Fprintln(w, initializBuildpacksCommitsResponse)

					case "/repos/initializ-community/other-example-repo/pulls/2/commits":
						w.WriteHeader(http.StatusOK)
						fmt.Fprintln(w, initializCommunityCommitsResponse)
					default:
						t.Fatal(fmt.Sprintf("unknown path: %s", req.URL.Path))
					}
				}))

				os.Setenv("PAT", "abcdefg")
			})

			it.After(func() {
				mockGithubServer.Close()
			})

			it("exits with the appropriate error", func() {

				command := exec.Command(mergeTimeCalculator, "--server", mockGithubServer.URL)
				fmt.Println(command)
				buffer := gbytes.NewBuffer()
				session, err := gexec.Start(command, buffer, buffer)

				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })

				out := string(buffer.Contents())

				Expect(out).To(ContainSubstring("failed to calculate merge times: failed to get closed pull requests:"))
			})
		})

		context("given there is an issue getting commits from a pull request", func() {
			it.Before(func() {
				mockGithubServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					if req.Method == http.MethodHead {
						http.Error(w, "NotFound", http.StatusNotFound)
						return
					}

					switch req.URL.Path {
					case "/orgs/initializ-buildpacks/repos":
						w.WriteHeader(http.StatusOK)
						fmt.Fprintln(w, initializBuildpacksRepoResponse)

					case "/orgs/initializ-community/repos":
						w.WriteHeader(http.StatusOK)
						fmt.Fprintln(w, initializCommunityRepoResponse)

					case "/repos/initializ-buildpacks/example-repo/pulls":
						w.WriteHeader(http.StatusOK)
						fmt.Fprintln(w, initializBuildpacksClosedPRsResponse)

					case "/repos/initializ-community/other-example-repo/pulls":
						w.WriteHeader(http.StatusOK)
						fmt.Fprintln(w, initializCommunityClosedPRsResponse)

					case "/repos/initializ-buildpacks/example-repo/pulls/1/commits":
						fmt.Fprintf(w, "unknown path: %s\n", req.URL.Path)

					case "/repos/initializ-community/other-example-repo/pulls/2/commits":
						w.WriteHeader(http.StatusOK)
						fmt.Fprintln(w, initializCommunityCommitsResponse)
					default:
						t.Fatal(fmt.Sprintf("unknown path: %s", req.URL.Path))
					}
				}))

				os.Setenv("PAT", "abcdefg")
			})

			it.After(func() {
				mockGithubServer.Close()
			})

			it("exits with the appropriate error", func() {

				command := exec.Command(mergeTimeCalculator, "--server", mockGithubServer.URL)
				fmt.Println(command)
				buffer := gbytes.NewBuffer()
				session, err := gexec.Start(command, buffer, buffer)

				Expect(err).NotTo(HaveOccurred())
				Eventually(session).Should(gexec.Exit(1), func() string { return string(buffer.Contents()) })

				out := string(buffer.Contents())

				Expect(out).To(ContainSubstring("failed to calculate merge times: failed to compute merge time for a pull request: could not get commits from closed pull request:"))
			})
		})
	})
}
