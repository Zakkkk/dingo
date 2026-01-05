package com.madappgang.dingo.lsp

import com.intellij.openapi.project.Project
import com.intellij.openapi.vfs.VirtualFile
import com.intellij.platform.lsp.api.LspServerSupportProvider
import com.intellij.platform.lsp.api.LspServerSupportProvider.LspServerStarter

/**
 * Entry point for Dingo LSP integration.
 * Registered via com.intellij.platform.lsp.serverSupportProvider extension point.
 */
class DingoLspServerSupportProvider : LspServerSupportProvider {

    override fun fileOpened(
        project: Project,
        file: VirtualFile,
        serverStarter: LspServerStarter
    ) {
        if (file.extension == "dingo") {
            serverStarter.ensureServerStarted(DingoLspServerDescriptor(project))
        }
    }
}
