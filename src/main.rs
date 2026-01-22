use clap::{Parser, Subcommand};

/// A fast, opinionated package manager that installs software directly onto your system
#[derive(Parser)]
#[command(name = "alloy")]
#[command(version, about, long_about = None)]
struct Cli {
    /// Run without making any changes to the system
    #[arg(long, global = true)]
    dry_run: bool,

    #[command(subcommand)]
    command: Commands,
}

#[derive(Subcommand)]
enum Commands {
    /// Install a package
    Install {
        /// Name of the package to install
        package: String,

        /// Specific version to install
        #[arg(short, long)]
        version: Option<String>,
    },

    /// Remove an installed package
    Remove {
        /// Name of the package to remove
        package: String,
    },

    /// List installed packages
    List {
        /// Show detailed information for each package
        #[arg(short, long)]
        verbose: bool,
    },

    /// Show information about a package
    Info {
        /// Name of the package
        package: String,
    },

    /// Check system health and diagnose issues
    Doctor,
}

fn main() {
    let cli = Cli::parse();

    if cli.dry_run {
        println!("[dry-run] No changes will be made to the system");
    }

    match cli.command {
        Commands::Install { package, version } => {
            cmd_install(&package, version.as_deref(), cli.dry_run);
        }
        Commands::Remove { package } => {
            cmd_remove(&package, cli.dry_run);
        }
        Commands::List { verbose } => {
            cmd_list(verbose);
        }
        Commands::Info { package } => {
            cmd_info(&package);
        }
        Commands::Doctor => {
            cmd_doctor();
        }
    }
}

fn cmd_install(package: &str, version: Option<&str>, dry_run: bool) {
    match version {
        Some(v) => println!("Installing {}@{}", package, v),
        None => println!("Installing {} (latest)", package),
    }
    if dry_run {
        println!("[dry-run] Would install {}", package);
    }
    // TODO: Implement installation logic
}

fn cmd_remove(package: &str, dry_run: bool) {
    println!("Removing {}", package);
    if dry_run {
        println!("[dry-run] Would remove {}", package);
    }
    // TODO: Implement removal logic
}

fn cmd_list(verbose: bool) {
    println!("Listing installed packages");
    if verbose {
        println!("(verbose mode)");
    }
    // TODO: Implement list logic
}

fn cmd_info(package: &str) {
    println!("Package info: {}", package);
    // TODO: Implement info logic
}

fn cmd_doctor() {
    println!("Running system health check...");
    // TODO: Implement doctor logic
}
