"""Main entry point for provider gateway."""

from app.__main__ import main
import asyncio
import sys

if __name__ == "__main__":
    try:
        exit_code = asyncio.run(main())
        sys.exit(exit_code)
    except KeyboardInterrupt:
        sys.exit(0)
