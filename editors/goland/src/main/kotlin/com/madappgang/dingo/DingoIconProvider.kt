package com.madappgang.dingo

import com.intellij.ide.IconProvider
import com.intellij.psi.PsiElement
import com.intellij.psi.PsiFile
import javax.swing.Icon

/**
 * Provides file icon for .dingo files.
 * Uses IconProvider instead of FileType to avoid conflicting with TextMate.
 */
class DingoIconProvider : IconProvider() {
    override fun getIcon(element: PsiElement, flags: Int): Icon? {
        if (element is PsiFile && element.virtualFile?.extension == "dingo") {
            return DingoIcons.FILE
        }
        return null
    }
}
