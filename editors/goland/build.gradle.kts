import org.jetbrains.kotlin.gradle.dsl.JvmTarget

plugins {
    id("org.jetbrains.kotlin.jvm") version "1.9.24"
    id("org.jetbrains.intellij.platform") version "2.10.5"
}

group = "com.madappgang"
version = "0.1.1"

repositories {
    mavenCentral()
    intellijPlatform {
        defaultRepositories()
    }
}

dependencies {
    intellijPlatform {
        // Target IntelliJ Ultimate 2024.1 for stable LSP and TextMate APIs
        // The plugin will work in GoLand, IDEA Ultimate, WebStorm, etc.
        // Community editions get syntax highlighting but not LSP features.
        intellijIdeaUltimate("2024.1")

        // Bundled plugins we depend on
        bundledPlugin("org.jetbrains.plugins.textmate")

        instrumentationTools()
    }
}

kotlin {
    jvmToolchain(17)
    compilerOptions {
        jvmTarget.set(JvmTarget.JVM_17)
    }
}

intellijPlatform {
    pluginConfiguration {
        name = "Dingo Language Support"
        ideaVersion {
            sinceBuild = "241"  // 2024.1
            untilBuild = provider { null }  // No upper limit
        }
    }

    buildSearchableOptions = false

    signing {
        // Configure for JetBrains Marketplace signing
        // certificateChain = providers.environmentVariable("CERTIFICATE_CHAIN")
        // privateKey = providers.environmentVariable("PRIVATE_KEY")
        // password = providers.environmentVariable("PRIVATE_KEY_PASSWORD")
    }

    publishing {
        // token = providers.environmentVariable("PUBLISH_TOKEN")
    }
}

tasks {
    wrapper {
        gradleVersion = "8.13"
    }

    patchPluginXml {
        sinceBuild = "241"
        untilBuild = provider { null }
    }

    runIde {
        // Use GoLand for testing if available
        // ideDir = file("/Applications/GoLand.app/Contents")
    }
}
