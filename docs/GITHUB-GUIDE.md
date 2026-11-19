# GitHub guide: SSH, first push, CI, releases

A complete walk-through from "I have the project folder" to "students download installers from a GitHub Release".
Commands are for Linux (Kubuntu / Rocky); Windows notes are marked **Windows**. Replace `YOUR-USER` with your GitHub user name.

```
your PC ──git push (SSH)──▶ GitHub repo ──▶ Actions CI (test + build + smoke-test on Ubuntu, Rocky, Windows)
                                   └─ push tag v0.0.3 ──▶ Release workflow ──▶ GitHub Release with .deb .rpm .exe + SHA256SUMS
```

## 1. One-time setup

### 1.1 Install git and set your identity
```bash
sudo apt install git            # Kubuntu/Debian   (Rocky: sudo dnf install git)
git config --global user.name  "Iqbal Uddin Khan"
git config --global user.email "you@example.com"      # use the e-mail on your GitHub account (or the noreply address from GitHub > Settings > Emails)
git config --global init.defaultBranch main
```
**Windows:** install *Git for Windows*; it includes Git Bash and OpenSSH. Run the commands below in Git Bash.

### 1.2 Create an SSH key
```bash
ssh-keygen -t ed25519 -C "you@example.com"        # press Enter for the default file; set a passphrase
eval "$(ssh-agent -s)" && ssh-add ~/.ssh/id_ed25519
cat ~/.ssh/id_ed25519.pub                          # copy the whole line
```
Never share the file without `.pub` (the private key).

### 1.3 Add the public key to GitHub
GitHub > profile picture > **Settings** > **SSH and GPG keys** > **New SSH key** > Title "Kubuntu laptop", Key type *Authentication Key*, paste the line > **Add SSH key**.

### 1.4 Test the connection
```bash
ssh -T git@github.com
```
The first time it asks to trust `github.com`. Compare the shown fingerprint with the list on GitHub's page
*"GitHub's SSH key fingerprints"* (docs.github.com) and answer `yes` only if they match.
Success looks like: `Hi YOUR-USER! You've successfully authenticated, but GitHub does not provide shell access.`

## 2. Create the repository on GitHub
1. GitHub > **+** > **New repository**.
2. Name: `aiot-simulator`. **Do not** tick *Add a README*, *.gitignore* or *license* (the project already has them; an initial commit would conflict).
3. Visibility: **Public** gives unlimited free Actions minutes. **Private** uses your plan's monthly minutes (Windows jobs count double) - fine for testing, but students then cannot download releases without access.
4. **Create repository**. Keep the page open; it shows the SSH URL `git@github.com:YOUR-USER/aiot-simulator.git`.

## 3. First push
```bash
tar -xzf aiot-simulator-repo.tar.gz && cd aiot-sim
git log --oneline                 # shows the project history
git branch -M main
git remote add origin git@github.com:YOUR-USER/aiot-simulator.git
git push -u origin main
git push origin --tags            # pushes the v0.0.x tags (see section 5 before pushing a tag you do not want released)
```
> The tag push **starts the Release workflow** for every `v*` tag that did not exist on GitHub yet. For the very first
> push this is what you want if the newest tag is the version you intend to publish. To push only the code first:
> `git push -u origin main` and push tags later.

**If you already pushed an older copy** and the history differs (for example you used `git commit --amend --reset-author`), do not force-push. Copy the new files into your existing clone instead:
```bash
rsync -a --delete --exclude .git /path/to/new/aiot-sim/ /path/to/your/clone/
cd /path/to/your/clone && git add -A && git commit -m "Update to v0.0.3" && git push
```

## 4. What happens after the push: Actions (CI)
Open the repo > **Actions** tab > workflow **CI**. It starts automatically for every push to `main` and every pull request.

| Job | Runs on | What it checks |
|---|---|---|
| `test` | ubuntu | `gofmt`, `go vet`, Go tests with the race detector, JavaScript syntax check, JavaScript tests (sketch interpreter, circuit model) |
| `package` | ubuntu | builds the DEB, RPM and Windows installer with `build.sh`, uploads them as the artifact **installers** (kept 14 days) |
| `smoke-deb` | ubuntu | installs the `.deb`, starts the app, checks the API answers, removes the package |
| `smoke-rpm` | Rocky Linux 9 container | installs the `.rpm`, starts the app, checks the API, removes it |
| `smoke-windows` | windows-latest | silent-installs the `.exe`, starts the app, checks the API, uninstalls |

Total time is roughly 5-8 minutes. A green tick on the commit means every installer installs, starts and answers.

**Download test builds:** Actions > the run > **Artifacts** > `installers` (a zip with the three installers).
**Run it by hand:** Actions > CI > **Run workflow** (the `workflow_dispatch` trigger).
**Add a status badge** to `README.md`:
`![CI](https://github.com/YOUR-USER/aiot-simulator/actions/workflows/ci.yml/badge.svg)`

### If a job is red
Click the job > the failing step > read the last 20 lines. Common causes:

| Symptom | Cause and fix |
|---|---|
| `gofmt` step fails | run `gofmt -w .` locally, commit |
| `go vet` / tests fail | run `make test` locally; fix; push |
| `package` fails at `build.sh` | run `./build.sh` locally (needs `golang-go dpkg-dev fakeroot rpm nsis python3-pil`) |
| `smoke-rpm`: `dnf` cannot find `xdg-utils` | the container could not reach the repo (temporary) - **Re-run failed jobs** |
| `smoke-windows`: "did not answer on port 8765" | antivirus/port clash on the runner - re-run once; if repeatable, open the job log and look for the app's crash text |
| workflow not starting | Settings > Actions > General > *Allow all actions*; check the file is in `.github/workflows/` and valid YAML |

After fixing: commit, `git push`; CI runs again. **Re-run all jobs** is also available on the run page.

## 5. Publishing a release (the installers students download)
The **Release** workflow runs when you push a tag that starts with `v`.

```bash
# 1. decide the version (semantic: MAJOR.MINOR.PATCH)
echo 0.0.4 > VERSION
# 2. describe the changes: add a section at the top of CHANGELOG.md
#      ## [0.0.4] - 2026-12-01
#      ### Added
#      - ...
# 3. commit and push, wait for the green CI tick
git add -A && git commit -m "Release 0.0.4" && git push
# 4. tag exactly that version and push the tag
git tag -a v0.0.4 -m "AIoT Simulator 0.0.4"
git push origin v0.0.4
```
What the workflow does (Actions > **Release**):
1. checks the tag matches `VERSION` (`v0.0.4` or `v0.0.4-rc1`); otherwise it stops with a clear error;
2. runs `build.sh`, which first runs vet, Go tests and JavaScript tests, then builds `.deb`, `.rpm`, `.exe` and `SHA256SUMS`;
3. copies the `## [0.0.4]` section of `CHANGELOG.md` into the release notes (plus GitHub's auto-generated list of changes);
4. creates the release at **Code > Releases** with all files attached.

**Pre-release:** tag `v0.1.0-rc1` (VERSION must be `0.1.0`). It is marked *Pre-release* automatically; packages become `0.1.0~rc1`.

**Students install** from Releases:
```bash
sha256sum -c SHA256SUMS --ignore-missing          # verify the download
sudo apt install ./aiot-simulator_0.0.4_amd64.deb   # Kubuntu / Debian
sudo dnf install ./aiot-simulator-0.0.4-1.x86_64.rpm # Rocky / Fedora
```
Windows: run `AIoT-Simulator-0.0.4-setup.exe` (SmartScreen: *More info > Run anyway*, because the installer is not code-signed).

### Fixing a wrong release
```bash
git tag -d v0.0.4 && git push origin :refs/tags/v0.0.4      # delete the tag locally and on GitHub
# also delete the release on GitHub: Releases > the release > Delete
# fix the problem, commit, push, then tag again
```
Prefer a new patch version (`0.0.5`) once people have downloaded a release.

## 6. Recommended repository settings
- **Settings > Actions > General > Workflow permissions:** *Read repository contents* is fine; the release workflow requests `contents: write` itself. If your organisation blocks that, switch to *Read and write*.
- **Settings > Branches > Add rule** for `main`: *Require a pull request* and *Require status checks to pass* (select `test`, `package`, `smoke-deb`, `smoke-rpm`, `smoke-windows`). Optional for a solo project.
- **Settings > Code security:** enable *Dependabot alerts*. `dependabot.yml` already proposes weekly updates for the actions used in the workflows.
- **Secrets:** none are needed; the workflows use the built-in `GITHUB_TOKEN`.
- **Releases > Edit** lets you add screenshots or a "How to install" paragraph after publishing.

## 7. Everyday workflow (recommended)
```bash
git switch -c feature/new-lab-panel     # work on a branch
# ...edit, test locally: make test
git add -A && git commit -m "Describe the change"
git push -u origin feature/new-lab-panel
# open a Pull Request on GitHub; CI runs; merge when green
git switch main && git pull
```
Then bump `VERSION`, update `CHANGELOG.md`, tag, push (section 5).

## 8. Troubleshooting SSH and git
| Message | Fix |
|---|---|
| `Permission denied (publickey)` | key not added to the agent (`ssh-add -l`; `ssh-add ~/.ssh/id_ed25519`) or public key not added on GitHub (1.3). Test with `ssh -vT git@github.com` |
| `Host key verification failed` | remove the stale entry: `ssh-keygen -R github.com`, reconnect, verify the fingerprint (1.4) |
| Asks for a user name/password | the remote is HTTPS. Switch: `git remote set-url origin git@github.com:YOUR-USER/aiot-simulator.git` |
| `Could not resolve hostname` / port 22 blocked (campus network) | use SSH over HTTPS port: add to `~/.ssh/config`:<br>`Host github.com`<br>`  Hostname ssh.github.com`<br>`  Port 443`<br>`  User git` |
| `rejected ... fetch first` / `non-fast-forward` | GitHub has commits you lack: `git pull --rebase`, then `git push`. Do not force-push a shared branch |
| `remote: Repository not found` | wrong user/repo name, or no access: check `git remote -v` |
| two GitHub accounts on one PC | create a second key and a `Host github-work` block in `~/.ssh/config` with `IdentityFile`, then use `git@github-work:ORG/repo.git` |
| shell scripts fail on **Windows** clones | caused by CRLF line endings; `.gitattributes` in this repo forces LF. Re-clone after pulling it |

## 9. Optional hardening (not configured)
- **Windows code signing** removes the SmartScreen warning (needs a certificate; free options exist for open-source projects via SignPath). Add a signing step to `release.yml` before the publish step.
- **Sign DEB/RPM** with a GPG key stored as a repository secret.
- **Build attestations** (`actions/attest-build-provenance`) let users verify that a file was built by your workflow.
- **arm64 packages** for Raspberry Pi: add `GOARCH=arm64` builds to `build.sh`.

## 10. Cheat sheet
```bash
ssh -T git@github.com                         # test SSH
git status / git log --oneline                # where am I
git push                                      # send commits (CI runs)
git tag -a vX.Y.Z -m "msg" && git push origin vX.Y.Z   # release
git tag -d vX.Y.Z && git push origin :refs/tags/vX.Y.Z # undo a tag
make test                                     # tests before pushing
./build.sh                                    # build installers locally into ./dist
```
