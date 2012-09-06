#! /bin/bash
conky -&
sleep 10
feh --bg-scale `grep 'wallpaper=' ~/.kde4/share/config/plasma-desktop-appletsrc | tail -n 2 | cut -b 11-`
