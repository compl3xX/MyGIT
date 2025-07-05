# MyGit - A Git Implementation in Go

This project is a simplified implementation of the Git version control system, written in Go. It's a great way to learn about the internal workings of Git, including how it stores objects, manages branches, and handles commands like `add`, `commit`, and `push`.

## How it Works

MyGit mimics the core concepts of Git, but with a simplified approach. Here's a breakdown of how it works, with comparisons to the real Git.

### The `.mygit` Directory

Just like Git has a `.git` directory, MyGit has a `.mygit` directory. This is where all the magic happens. It contains the following:

- **`objects`**: This is the object database. It stores all the "objects" that make up your project's history, including commits, trees, and blobs (file content).
- **`refs`**: This directory stores references to commits, such as branches and tags.
- **`HEAD`**: This file points to the current branch or commit you're working on.
- **`index`**: This is the staging area. It's a list of all the files that are ready to be committed.

### Objects

MyGit uses the same three types of objects as Git:

- **Blobs**: These store the content of your files.
- **Trees**: These represent directories. They contain a list of other trees and blobs.
- **Commits**: These represent a snapshot of your project at a specific point in time. They contain a reference to a tree, the author, a commit message, and one or more parent commits.

#### How it's different from Git

- **Hashing**: MyGit uses SHA-1 to hash objects, just like Git. However, the real Git has a more complex object database that uses packfiles to save space. MyGit stores each object as a separate file.
- **Deltas**: MyGit has a basic implementation of delta compression, which is used to reduce the size of packfiles. The real Git has a much more sophisticated delta compression algorithm.

### The Index

The index, or staging area, is a key concept in Git. It's a list of all the files that are ready to be committed. When you run `mygit add`, you're adding files to the index. When you run `mygit commit`, you're creating a new commit from the files in the index.

#### How it's different from Git

- **Implementation**: MyGit's index is a simple text file that lists the path, hash, and other metadata for each file. The real Git has a more complex binary index format.

## Commands

### `init`

Initializes a new MyGit repository in the current directory.

**How it's different from Git:**
- The real `git init` has more options, such as creating a bare repository.
- MyGit creates a `.mygit` directory, while Git creates a `.git` directory.

### `add`

Adds file contents to the index.

**How it's different from Git:**
- `mygit add` can take a file or a directory as an argument.
- The real `git add` has more options, such as adding files interactively.

### `commit`

Records changes to the repository.

**How it's different from Git:**
- `mygit commit` only supports the `-m` flag for providing a commit message.
- The real `git commit` has many more options, such as amending the previous commit, signing commits, and more.

### `log`

Shows the commit logs.

**How it's different from Git:**
- `mygit log` shows a simplified view of the commit history.
- The real `git log` has a vast number of options for formatting and filtering the output.

### `status`

Shows the working tree status.

**How it's different from Git:**
- `mygit status` provides a basic overview of the repository's state.
- The real `git status` has a more detailed and configurable output.

### `branch`

Lists, creates, or deletes branches.

**How it's different from Git:**
- `mygit branch` only supports listing and creating branches.
- The real `git branch` has many more options, such as renaming branches, setting up tracking information, and more.

### `checkout`

Switches branches or restores working tree files.

**How it's different from Git:**
- `mygit checkout` only supports switching branches.
- The real `git checkout` has many more options, such as creating new branches, detaching HEAD, and restoring files from a specific commit.

### `push`

Updates remote refs along with associated objects.

**How it's different from Git:**
- `mygit push` only supports pushing to a remote repository over HTTPS.
- The real `git push` supports multiple protocols (SSH, Git, etc.) and has many more options for controlling how branches are pushed.

### `show`

Shows various types of objects.

**How it's different from Git:**
- `mygit show` provides a basic view of an object's contents.
- The real `git show` has many more options for formatting the output.

## Examples

Here's a comparison of how you would use MyGit versus the real Git:

| Action | MyGit | Git |
| --- | --- | --- |
| Initialize a repository | `mygit init` | `git init` |
| Add a file to the index | `mygit add README.md` | `git add README.md` |
| Commit your changes | `mygit commit -m "Initial commit"` | `git commit -m "Initial commit"` |
| Push to a remote | `mygit push origin main` | `git push origin main` |

## Conclusion

MyGit is a great way to learn about the internal workings of Git. While it's not a full-featured replacement for the real Git, it's a fun and educational project that can help you to better understand how your favorite version control system works.