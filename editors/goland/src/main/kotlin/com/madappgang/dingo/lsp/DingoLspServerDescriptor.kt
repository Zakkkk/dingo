package com.madappgang.dingo.lsp

import com.intellij.execution.configurations.GeneralCommandLine
import com.intellij.execution.configurations.PathEnvironmentVariableUtil
import com.intellij.openapi.diagnostic.thisLogger
import com.intellij.openapi.project.Project
import com.intellij.openapi.vfs.VirtualFile
import com.intellij.platform.lsp.api.ProjectWideLspServerDescriptor
import java.io.File

/**
 * Configures the Dingo language server connection.
 *
 * The server is expected to be available in PATH as 'dingo-lsp'.
 * Users install it via: go install github.com/MadAppGang/dingo/cmd/dingo-lsp@latest
 */
class DingoLspServerDescriptor(project: Project) :
    ProjectWideLspServerDescriptor(project, "Dingo") {

    override fun isSupportedFile(file: VirtualFile): Boolean {
        return file.extension == "dingo" && file.isValid && file.isInLocalFileSystem
    }

    override fun createCommandLine(): GeneralCommandLine {
        val lspPath = findDingoLsp()
        return GeneralCommandLine(lspPath).apply {
            withWorkDirectory(project.basePath ?: System.getProperty("user.dir"))
            withParentEnvironmentType(GeneralCommandLine.ParentEnvironmentType.CONSOLE)
        }
    }

    /**
     * Find dingo-lsp executable.
     * Search order:
     * 1. System PATH (using IntelliJ platform utility for cross-platform support)
     * 2. GOPATH/bin
     * 3. ~/go/bin (default GOPATH)
     */
    private fun findDingoLsp(): String {
        // Platform-agnostic binary name (handles .exe on Windows automatically)
        val exeName = if (System.getProperty("os.name").lowercase().contains("windows")) {
            "dingo-lsp.exe"
        } else {
            "dingo-lsp"
        }

        // Use IntelliJ utility for cross-platform PATH search (handles .exe on Windows)
        val fromPath = PathEnvironmentVariableUtil.findInPath("dingo-lsp")
        if (fromPath != null && fromPath.canExecute()) {
            return fromPath.absolutePath
        }

        // Check GOPATH/bin
        val gopath = System.getenv("GOPATH")
        if (gopath != null) {
            val lsp = File(gopath, "bin/$exeName")
            if (lsp.exists() && lsp.canExecute()) {
                return lsp.absolutePath
            }
        }

        // Check ~/go/bin (default GOPATH)
        val homeGoLsp = File(System.getProperty("user.home"), "go/bin/$exeName")
        if (homeGoLsp.exists() && homeGoLsp.canExecute()) {
            return homeGoLsp.absolutePath
        }

        // Fallback: log warning and return basename (will fail with clearer error)
        thisLogger().warn(
            "dingo-lsp not found in PATH, GOPATH/bin, or ~/go/bin. " +
            "Install via: go install github.com/MadAppGang/dingo/cmd/dingo-lsp@latest"
        )
        return exeName
    }
}
