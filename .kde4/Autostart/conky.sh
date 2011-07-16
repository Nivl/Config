#! /bin/bash
conky -&
sleep 3
feh --bg-scale `grep 'wallpaper=' ~/.kde4/share/config/plasma-desktop-appletsrc | tail --bytes=+11`
