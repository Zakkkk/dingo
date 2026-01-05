package com.madappgang.dingo

import com.intellij.openapi.application.PathManager
import com.intellij.openapi.diagnostic.Logger
import org.jetbrains.plugins.textmate.api.TextMateBundleProvider
import java.nio.file.Files
import java.nio.file.Path

/**
 * Provides TextMate grammar for Dingo syntax highlighting.
 * The grammar is reused from the VS Code extension.
 */
class DingoTextMateBundle : TextMateBundleProvider {

    companion object {
        private val LOG = Logger.getInstance(DingoTextMateBundle::class.java)
        private val BUNDLE_FILES = listOf(
            "package.json",
            "syntaxes/dingo.tmLanguage.json"
        )
    }

    override fun getBundles(): List<TextMateBundleProvider.PluginBundle> {
        LOG.info("DingoTextMateBundle.getBundles() called")
        return try {
            val bundleDir = Files.createTempDirectory(
                Path.of(PathManager.getTempPath()), "textmate-dingo"
            )

            for (fileToCopy in BUNDLE_FILES) {
                val resource = DingoTextMateBundle::class.java.classLoader
                    .getResource("textmate/dingo/$fileToCopy")
                if (resource == null) {
                    LOG.error("TextMate resource not found: textmate/dingo/$fileToCopy")
                    continue
                }
                resource.openStream().use { inputStream ->
                    val target = bundleDir.resolve(fileToCopy)
                    Files.createDirectories(target.parent)
                    Files.copy(inputStream, target)
                    LOG.info("Extracted: $fileToCopy -> $target")
                }
            }

            LOG.info("Dingo TextMate bundle created at: $bundleDir")
            listOf(TextMateBundleProvider.PluginBundle("Dingo", bundleDir))
        } catch (e: Exception) {
            LOG.error("Failed to create TextMate bundle", e)
            emptyList()
        }
    }
}
