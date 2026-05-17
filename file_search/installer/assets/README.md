# installer/assets/

Place the following files here before compiling installer.iss:

## Required
- icon.ico         — 256x256 app icon (ICO format, multi-resolution)
- wizard_small.bmp — 55x58 pixels, 24-bit BMP (shown in wizard sidebar)

## Tips
- Convert PNG to ICO: https://www.icoconverter.com/
- Use a simple magnifying glass or file icon for FileSearch
- wizard_small.bmp can be a simple white/blue branded image

## If you don't have assets yet
1. Remove or comment these lines in installer.iss:
   SetupIconFile=assets\icon.ico
   WizardSmallImageFile=assets\wizard_small.bmp

2. Inno Setup will use its default icon instead.
