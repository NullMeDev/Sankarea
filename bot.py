import discord
from discord.ext import commands, tasks
import yaml
import json
import os
import asyncio
import feedparser
from datetime import datetime, timezone
import logging
from pathlib import Path

# Set up logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

class RSSBot(commands.Bot):
    def __init__(self):
        intents = discord.Intents.default()
        intents.message_content = True
        intents.members = True
        
        super().__init__(
            command_prefix='!',
            intents=intents,
            help_command=commands.DefaultHelpCommand()
        )
        
        # Load configuration
        self.config = self.load_config()
        
        # Initialize feed cache
        self.feed_cache = {}
        self.last_post_times = {}
        
        # Register commands
        self.setup_commands()
        
    async def setup_hook(self):
        """Setup hook that runs when the bot is first starting"""
        # Start the feed check loop
        self.check_feeds.start()
        
    def load_config(self):
        """Load configuration from config files"""
        try:
            # Load RSS sources
            with open('config/sources.yml', 'r') as f:
                sources = yaml.safe_load(f)
            
            # Load bot configuration
            with open('config/bot_config.yml', 'r') as f:
                bot_config = yaml.safe_load(f)
                
            return {
                'sources': sources,
                'bot': bot_config
            }
        except Exception as e:
            logger.error(f"Failed to load configuration: {e}")
            return None

    def setup_commands(self):
        """Setup bot commands"""
        
        @self.command(name='ping')
        async def ping(ctx):
            """Check if the bot is responsive"""
            await ctx.send(f'Pong! Latency: {round(self.latency * 1000)}ms')

        @self.command(name='sources')
        async def list_sources(ctx):
            """List all available RSS sources"""
            sources = self.config['sources']['sources']
            categories = {}
            
            # Group sources by category
            for source in sources:
                category = source['category']
                if category not in categories:
                    categories[category] = []
                categories[category].append(source['name'])
            
            # Create embed
            embed = discord.Embed(
                title="Available RSS Sources",
                color=discord.Color.blue()
            )
            
            # Add fields for each category
            for category, sources in categories.items():
                embed.add_field(
                    name=category,
                    value="\n".join(sources),
                    inline=False
                )
            
            await ctx.send(embed=embed)

    @tasks.loop(minutes=30)
    async def check_feeds(self):
        """Check RSS feeds periodically"""
        try:
            if not self.config:
                logger.error("No configuration loaded")
                return

            for source in self.config['sources']['sources']:
                try:
                    # Parse feed
                    feed = await self.fetch_feed(source['url'])
                    
                    if not feed:
                        continue
                    
                    # Get channel for this category
                    channel_id = self.config['bot']['category_channels'].get(source['category'])
                    if not channel_id:
                        continue
                        
                    channel = self.get_channel(channel_id)
                    if not channel:
                        continue
                    
                    # Process new entries
                    await self.process_feed_entries(feed, source, channel)
                    
                except Exception as e:
                    logger.error(f"Error processing feed {source['name']}: {e}")
                    
                # Add delay between feeds to avoid rate limiting
                await asyncio.sleep(1)
                
        except Exception as e:
            logger.error(f"Error in check_feeds: {e}")

    async def fetch_feed(self, url):
        """Fetch and parse RSS feed"""
        try:
            feed = feedparser.parse(url)
            if feed.bozo:
                logger.error(f"Feed error for {url}: {feed.bozo_exception}")
                return None
            return feed
        except Exception as e:
            logger.error(f"Error fetching feed {url}: {e}")
            return None

    async def process_feed_entries(self, feed, source, channel):
        """Process new entries from a feed"""
        try:
            # Get last post time for this feed
            last_post_time = self.last_post_times.get(source['url'], datetime.min.replace(tzinfo=timezone.utc))
            
            for entry in feed.entries[:5]:  # Process up to 5 newest entries
                # Get entry timestamp
                published = self.parse_entry_time(entry)
                
                if published and published > last_post_time:
                    # Create embed for the entry
                    embed = self.create_feed_embed(entry, source)
                    
                    # Send to channel
                    await channel.send(embed=embed)
                    
                    # Update last post time
                    self.last_post_times[source['url']] = published
                    
        except Exception as e:
            logger.error(f"Error processing entries for {source['name']}: {e}")

    def parse_entry_time(self, entry):
        """Parse entry timestamp"""
        for time_field in ['published_parsed', 'updated_parsed']:
            if hasattr(entry, time_field):
                time_tuple = getattr(entry, time_field)
                if time_tuple:
                    return datetime(*time_tuple[:6], tzinfo=timezone.utc)
        return None

    def create_feed_embed(self, entry, source):
        """Create Discord embed for feed entry"""
        embed = discord.Embed(
            title=entry.get('title', 'No Title'),
            url=entry.get('link', ''),
            color=discord.Color.blue(),
            description=entry.get('summary', '')[:2000]  # Discord's description limit
        )
        
        embed.set_footer(text=f"Source: {source['name']}")
        
        if hasattr(entry, 'published'):
            embed.timestamp = self.parse_entry_time(entry) or datetime.now(timezone.utc)
            
        return embed

def main():
    """Main entry point"""
    bot = RSSBot()
    
    @bot.event
    async def on_ready():
        logger.info(f'Logged in as {bot.user.name} ({bot.user.id})')
        
    # Run the bot
    bot.run(os.getenv('DISCORD_TOKEN'))

if __name__ == '__main__':
    main()
