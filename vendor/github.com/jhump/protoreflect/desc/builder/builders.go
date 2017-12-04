package builder

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync/atomic"
	"unicode"

	"github.com/golang/protobuf/proto"
	dpb "github.com/golang/protobuf/protoc-gen-go/descriptor"

	"github.com/jhump/protoreflect/desc"
	. "github.com/jhump/protoreflect/desc/internal"
)

// TODO: finish writing Go doc for all types and methods

// Builder is the core interface implemented by all descriptor builders. It
// exposes some basic information about the descriptor hierarchy's structure.
//
// All Builders also have a Build() method, but that is not part of this
// interface because its return type varies with the type of descriptor that
// is built.
type Builder interface {
	// GetName returns this element's name. The name returned is a simple name,
	// not a qualified name.
	GetName() string

	// TrySetName attempts to set this element's name. If the rename cannot
	// proceed (e.g. this element's parent already has an element with that
	// name) then an error is returned.
	//
	// All builders also have a method named SetName that panics on error and
	// returns the builder itself (for method chaining). But that isn't defined
	// on this interface because its return type varies with the type of the
	// descriptor builder.
	TrySetName(newName string) error

	// GetParent returns this element's parent element. It returns nil if there
	// is no parent element. File builders never have parent elements.
	GetParent() Builder

	// GetFile returns this element's file. This returns nil if the element has
	// not yet been assigned to a file.
	GetFile() *FileBuilder

	// GetChildren returns all of this element's child elements. A file will
	// return all of its top-level messages, enums, extensions, and services. A
	// message will return all of its fields as well as nested messages, enums,
	// and extensions, etc. Children will generally be grouped by type and,
	// within a group, in the same order as the children were added to their
	// parent.
	GetChildren() []Builder

	// GetComments returns the comments for this element. If the element has no
	// comments then the returned struct will have all empty fields. Comments
	// can be added to the element by setting fields of the returned struct.
	//
	// All builders also have a SetComments method that modifies the comments
	// and returns the builder itself (for method chaining). But that isn't
	// defined on this interface because its return type varies with the type of
	// the descriptor builder.
	GetComments() *Comments

	// findChild returns the child builder with the given name or nil if this
	// builder has no such child.
	findChild(string) Builder

	// removeChild removes the given child builder from this element. If the
	// given element is not a child, it should do nothing.
	//
	// NOTE: It is this method's responsibility to call child.setParent(nil)
	// after removing references to the child from this element.
	removeChild(Builder)

	// renamedChild updates references by-name references to the given child and
	// validates its name. The given string is the child's old name. If the
	// rename can proceed, no error should be returned and any by-name
	// references to the old name should be removed.
	renamedChild(Builder, string) error

	// setParent simply updates the up-link (from child to parent) so that the
	// this element's parent is up-to-date. It does NOT try to remove references
	// from the parent to this child. (See doc for removeChild(Builder)).
	setParent(Builder)
}

// Comments represents the various comments that might be associated with a
// descriptor. These are equivalent to the various kinds of comments found in a
// *dpb.SourceCodeInfo_Location struct that protoc associates with elements in
// the parsed proto source file. This can be used to create or preserve comments
// (including documentation) for elements.
type Comments struct {
	LeadingDetachedComments []string
	LeadingComment          string
	TrailingComment         string
}

func setComments(c *Comments, loc *dpb.SourceCodeInfo_Location) {
	c.LeadingDetachedComments = loc.GetLeadingDetachedComments()
	c.LeadingComment = loc.GetLeadingComments()
	c.TrailingComment = loc.GetTrailingComments()
}

func addCommentsTo(sourceInfo *dpb.SourceCodeInfo, path []int32, c *Comments) {
	var lead, trail *string
	if c.LeadingComment != "" {
		lead = proto.String(c.LeadingComment)
	}
	if c.TrailingComment != "" {
		trail = proto.String(c.TrailingComment)
	}

	// we need defensive copies of the slices
	p := make([]int32, len(path))
	copy(p, path)

	var detached []string
	if len(c.LeadingDetachedComments) > 0 {
		detached := make([]string, len(c.LeadingDetachedComments))
		copy(detached, c.LeadingDetachedComments)
	}

	sourceInfo.Location = append(sourceInfo.Location, &dpb.SourceCodeInfo_Location{
		LeadingDetachedComments: detached,
		LeadingComments:         lead,
		TrailingComments:        trail,
		Path:                    p,
		Span:                    []int32{0, 0, 0},
	})
}

/* NB: There are a few flows that need to maintain strong referential integrity
 * and perform symbol and/or number uniqueness checks. The way these flows are
 * implemented is described below. The actions generally involve two different
 * components: making local changes to an element and making corresponding
 * and/or related changes in a parent element. Below describes the separation of
 * responsibilities between the two.
 *
 *
 * RENAMING AN ELEMENT
 *
 * Renaming an element is initiated via Builder.TrySetName. Implementations
 * should do the following:
 *  1. Validate the new name using any local constraints and naming rules.
 *  2. If there are child elements whose names should be kept in sync in some
 *     way, rename them.
 *  3. Invoke baseBuilder.setName. This changes this element's name and then
 *     invokes Builder.renamedChild(child, oldName) to update any by-name
 *     references from the parent to the child.
 *  4. If step #3 failed, any other element names that were changed to keep
 *     them in sync (from step #2) should be reverted.
 *
 * A key part of this flow is how parents react to child elements being renamed.
 * This is done in Builder.renamedChild. Implementations should do the
 * following:
 *  1. Validate the name using any local constraints. (Often there are no new
 *     constraints and any checks already done by Builder.TrySetName should
 *     suffice.)
 *  2. If the parent element should be renamed to keep it in sync with the
 *     child's name, rename it.
 *  3. Register references to the element using the new name. A possible cause
 *     of error in this step is a uniqueness constraint, e.g. the element's new
 *     name collides with a sibling element's name.
 *  4. If step #3 failed and this element name was changed to keep it in sync
 *     (from step #2), it should be reverted.
 *  5. Finally, remove references to the element for the old name. This step
 *     should always succeed.
 *
 * Changing the tag number for a non-extension field has a similar flow since it
 * is also checked for uniqueness, to make sure the new tag number does not
 * conflict with another existing field.
 *
 * Note that TrySetName and renamedChild methods both can return an error, which
 * should indicate why the element could not be renamed (e.g. name is invalid,
 * new name conflicts with existing sibling names, etc).
 *
 *
 * MOVING/REMOVING AN ELEMENT
 *
 * When an element is added to a new parent but is already assigned to a parent,
 * it is "moved" to the new parent. This is done via "Add" methods on the parent
 * entity (for example, MessageBuilder.AddField). Implementations of such a
 * method should do the following:
 *  1. Register references to the element. A possible cause of failure in this
 *     step is that the new element collides with an existing child.
 *  2. Use the Unlink function to remove the element from any existing parent.
 *  3. Use Builder.setParent to link the child to its parent.
 *
 * The Unlink function, which removes an element from its parent if it has a
 * parent, relies on the parent's Builder.removeChild method. Implementations of
 * that method should do the following:
 *  1. Check that the element is actually a child. If not, return without doing
 *     anything.
 *  2. Remove all references to the child.
 *  3. Finally, this method must call Builder.setParent(nil) to clear the
 *     element's up-link so it no longer refers to the old parent.
 *
 * The "Add" methods typically have a "Try" form which can return an error. This
 * could happen if the new child is not legal to add (including, for example,
 * that its name collides with an existing child element).
 *
 * The removeChild and setParent methods, on the other hand, cannot return an
 * error and thus must always succeed.
 */

// baseBuilder is a struct that can be embedded into each Builder implementation
// and provides a kernel of builder-wiring support (to reduce boiler-plate in
// each implementation).
type baseBuilder struct {
	name     string
	parent   Builder
	comments Comments
}

func baseBuilderWithName(name string) baseBuilder {
	if err := checkName(name); err != nil {
		panic(err)
	}
	return baseBuilder{name: name}
}

func checkName(name string) error {
	for i, ch := range name {
		if i == 0 {
			if ch != '_' && (ch < 'a' || ch > 'z') && (ch < 'A' || ch > 'Z') {
				return fmt.Errorf("name %q is invalid; It must start with an underscore or letter", name)
			}
		} else {
			if ch != '_' && (ch < '0' || ch > '9') && (ch < 'a' || ch > 'z') && (ch < 'A' || ch > 'Z') {
				return fmt.Errorf("name %q contains invalid character %q; only underscores, letters, and numbers are allowed", name, string(ch))
			}
		}
	}
	return nil
}

func (b *baseBuilder) GetName() string {
	return b.name
}

func (b *baseBuilder) setName(fullBuilder Builder, newName string) error {
	if newName == b.name {
		return nil // no change
	}
	if err := checkName(newName); err != nil {
		return err
	}
	oldName := b.name
	b.name = newName
	if b.parent != nil {
		if err := b.parent.renamedChild(fullBuilder, oldName); err != nil {
			// revert the rename on error
			b.name = oldName
			return err
		}
	}
	return nil
}

func (b *baseBuilder) GetParent() Builder {
	return b.parent
}

func (b *baseBuilder) setParent(newParent Builder) {
	b.parent = newParent
}

func (b *baseBuilder) GetFile() *FileBuilder {
	var p Builder = b.parent
	for p != nil {
		if fb, ok := p.(*FileBuilder); ok {
			return fb
		}
		p = p.GetParent()
	}
	return nil
}

func (b *baseBuilder) GetComments() *Comments {
	return &b.comments
}

// doBuild is a helper for implementing the Build() method that each builder
// exposes. It is used for all builders except for the root FileBuilder type.
func doBuild(b Builder) (desc.Descriptor, error) {
	fd, err := newResolver().resolveElement(b, nil)
	if err != nil {
		return nil, err
	}
	return fd.FindSymbol(GetFullyQualifiedName(b)), nil
}

func getFullyQualifiedName(b Builder, buf *bytes.Buffer) {
	if fb, ok := b.(*FileBuilder); ok {
		buf.WriteString(fb.Package)
	} else if b != nil {
		p := b.GetParent()
		if _, ok := p.(*FieldBuilder); ok {
			// field can be the parent of a message (if it's
			// the field's map entry or group type), but its
			// name is not part of message's fqn; so skip
			p = p.GetParent()
		}
		if _, ok := p.(*OneOfBuilder); ok {
			// one-of can be the parent of a field, but its
			// name is not part of field's fqn; so skip
			p = p.GetParent()
		}
		getFullyQualifiedName(p, buf)
		if buf.Len() > 0 {
			buf.WriteByte('.')
		}
		buf.WriteString(b.GetName())
	}
}

// GetFullyQualifiedName returns the given builder's fully-qualified name. This
// name is based on the parent elements the builder may be linked to, which
// provide context like package and (optional) enclosing message names.
func GetFullyQualifiedName(b Builder) string {
	var buf bytes.Buffer
	getFullyQualifiedName(b, &buf)
	return buf.String()
}

// Unlink removes the given builder from its parent. The parent will no longer
// refer to the builder and vice versa.
func Unlink(b Builder) {
	if p := b.GetParent(); p != nil {
		p.removeChild(b)
	}
}

// getRoot navigates up the hierarchy to find the root builder for the given
// instance.
func getRoot(b Builder) Builder {
	for {
		p := b.GetParent()
		if p == nil {
			return b
		}
		b = p
	}
}

// deleteBuilder will delete a descriptor builder with the given name from the
// given slice. The slice's elements can be any builder type. The parameter has
// type interface{} so it can accept []*MessageBuilder or []*FieldBuilder, for
// example. It returns a value of the same type with the named builder omitted.
func deleteBuilder(name string, descs interface{}) interface{} {
	rv := reflect.ValueOf(descs)
	for i := 0; i < rv.Len(); i++ {
		c := rv.Index(i).Interface().(Builder)
		if c.GetName() == name {
			head := rv.Slice(0, i)
			tail := rv.Slice(i+1, rv.Len())
			return reflect.AppendSlice(head, tail)
		}
	}
	return descs
}

var uniqueFileCounter uint64

func uniqueFileName() string {
	i := atomic.AddUint64(&uniqueFileCounter, 1)
	return fmt.Sprintf("{generated-file-%04x}.proto", i)
}

func makeUnique(name string, existingNames map[string]struct{}) string {
	i := 1
	n := name
	for {
		if _, ok := existingNames[n]; !ok {
			return n
		}
		n = fmt.Sprintf("%s(%d)", name, i)
		i++
	}
}

// FileBuilder is a builder used to construct a desc.FileDescriptor. This is the
// root of the hierarchy. All other descriptors belong to a file, and thus all
// other builders also belong to a file.
//
// If a builder is *not* associated with a file, the resulting descriptor will
// be associated with a synthesized file that contains only the built descriptor
// and its ancestors. This means that such descriptors will have no associated
// package name.
type FileBuilder struct {
	name string

	IsProto3 bool
	Package  string
	Options  *dpb.FileOptions

	comments        Comments
	SyntaxComments  Comments
	PackageComments Comments

	messages   []*MessageBuilder
	extensions []*FieldBuilder
	enums      []*EnumBuilder
	services   []*ServiceBuilder
	symbols    map[string]Builder
}

// NewFile creates a new FileBuilder for a file with the given name. The
// name can be blank, which indicates a unique name should be generated for it.
func NewFile(name string) *FileBuilder {
	return &FileBuilder{
		name:    name,
		symbols: map[string]Builder{},
	}
}

// FromFile returns a FileBuilder that is effectively a copy of the given
// descriptor. (Note that builders do not retain source code info, even if the
// given descriptor included it.)
func FromFile(fd *desc.FileDescriptor) (*FileBuilder, error) {
	fb := NewFile(fd.GetName())
	fb.IsProto3 = fd.IsProto3()
	fb.Package = fd.GetPackage()
	fb.Options = fd.GetFileOptions()
	setComments(&fb.comments, fd.GetSourceInfo())

	// find syntax and package comments, too
	for _, loc := range fd.AsFileDescriptorProto().GetSourceCodeInfo().GetLocation() {
		if len(loc.Path) == 1 {
			if loc.Path[0] == File_syntaxTag {
				setComments(&fb.SyntaxComments, loc)
			} else if loc.Path[0] == File_packageTag {
				setComments(&fb.PackageComments, loc)
			}
		}
	}

	localMessages := map[*desc.MessageDescriptor]*MessageBuilder{}
	localEnums := map[*desc.EnumDescriptor]*EnumBuilder{}

	for _, md := range fd.GetMessageTypes() {
		if mb, err := fromMessage(md, localMessages, localEnums); err != nil {
			return nil, err
		} else if err := fb.TryAddMessage(mb); err != nil {
			return nil, err
		}
	}
	for _, ed := range fd.GetEnumTypes() {
		if eb, err := fromEnum(ed, localEnums); err != nil {
			return nil, err
		} else if err := fb.TryAddEnum(eb); err != nil {
			return nil, err
		}
	}
	for _, exd := range fd.GetExtensions() {
		if exb, err := fromField(exd); err != nil {
			return nil, err
		} else if err := fb.TryAddExtension(exb); err != nil {
			return nil, err
		}
	}
	for _, sd := range fd.GetServices() {
		if sb, err := fromService(sd); err != nil {
			return nil, err
		} else if err := fb.TryAddService(sb); err != nil {
			return nil, err
		}
	}

	// we've converted everything, so now we update all foreign type references
	// to be local type references if possible
	for _, mb := range fb.messages {
		updateLocalRefsInMessage(mb, localMessages, localEnums)
	}
	for _, exb := range fb.extensions {
		updateLocalRefsInField(exb, localMessages, localEnums)
	}
	for _, sb := range fb.services {
		for _, mtb := range sb.methods {
			updateLocalRefsInRpcType(mtb.ReqType, localMessages)
			updateLocalRefsInRpcType(mtb.RespType, localMessages)
		}
	}

	return fb, nil
}

func updateLocalRefsInMessage(mb *MessageBuilder, localMessages map[*desc.MessageDescriptor]*MessageBuilder, localEnums map[*desc.EnumDescriptor]*EnumBuilder) {
	for _, b := range mb.fieldsAndOneOfs {
		if flb, ok := b.(*FieldBuilder); ok {
			updateLocalRefsInField(flb, localMessages, localEnums)
		} else {
			oob := b.(*OneOfBuilder)
			for _, flb := range oob.choices {
				updateLocalRefsInField(flb, localMessages, localEnums)
			}
		}
	}
	for _, nmb := range mb.nestedMessages {
		updateLocalRefsInMessage(nmb, localMessages, localEnums)
	}
	for _, exb := range mb.nestedExtensions {
		updateLocalRefsInField(exb, localMessages, localEnums)
	}
}

func updateLocalRefsInField(flb *FieldBuilder, localMessages map[*desc.MessageDescriptor]*MessageBuilder, localEnums map[*desc.EnumDescriptor]*EnumBuilder) {
	if flb.fieldType.foreignMsgType != nil {
		if mb, ok := localMessages[flb.fieldType.foreignMsgType]; ok {
			flb.fieldType.foreignMsgType = nil
			flb.fieldType.localMsgType = mb
		}
	}
	if flb.fieldType.foreignEnumType != nil {
		if eb, ok := localEnums[flb.fieldType.foreignEnumType]; ok {
			flb.fieldType.foreignEnumType = nil
			flb.fieldType.localEnumType = eb
		}
	}
	if flb.foreignExtendee != nil {
		if mb, ok := localMessages[flb.foreignExtendee]; ok {
			flb.foreignExtendee = nil
			flb.localExtendee = mb
		}
	}
	if flb.msgType != nil {
		updateLocalRefsInMessage(flb.msgType, localMessages, localEnums)
	}
}

func updateLocalRefsInRpcType(rpcType *RpcType, localMessages map[*desc.MessageDescriptor]*MessageBuilder) {
	if rpcType.foreignType != nil {
		if mb, ok := localMessages[rpcType.foreignType]; ok {
			rpcType.foreignType = nil
			rpcType.localType = mb
		}
	}
}

func (fb *FileBuilder) GetName() string {
	return fb.name
}

// SetName changes this file's name, returning the file for method chaining. If
// there is an error that prevents the rename from succeeding, this method will
// panic.
func (fb *FileBuilder) SetName(newName string) *FileBuilder {
	fb.name = newName
	return fb
}

func (fb *FileBuilder) TrySetName(newName string) error {
	fb.name = newName
	return nil
}

func (fb *FileBuilder) GetParent() Builder {
	return nil
}

func (fb *FileBuilder) setParent(parent Builder) {
	if parent != nil {
		panic("files cannot have parent elements")
	}
}

func (fb *FileBuilder) GetComments() *Comments {
	return &fb.comments
}

func (fb *FileBuilder) SetComments(c Comments) *FileBuilder {
	fb.comments = c
	return fb
}

func (fb *FileBuilder) SetSyntaxComments(c Comments) *FileBuilder {
	fb.SyntaxComments = c
	return fb
}

func (fb *FileBuilder) SetPackageComments(c Comments) *FileBuilder {
	fb.PackageComments = c
	return fb
}

// GetFile implements the Builder interface and always returns this file.
func (fb *FileBuilder) GetFile() *FileBuilder {
	return fb
}

func (fb *FileBuilder) GetChildren() []Builder {
	var ch []Builder
	for _, mb := range fb.messages {
		ch = append(ch, mb)
	}
	for _, exb := range fb.extensions {
		ch = append(ch, exb)
	}
	for _, eb := range fb.enums {
		ch = append(ch, eb)
	}
	for _, sb := range fb.services {
		ch = append(ch, sb)
	}
	return ch
}

func (fb *FileBuilder) findChild(name string) Builder {
	return fb.symbols[name]
}

func (fb *FileBuilder) removeChild(b Builder) {
	if p, ok := b.GetParent().(*FileBuilder); !ok || p != fb {
		return
	}

	switch b.(type) {
	case *MessageBuilder:
		fb.messages = deleteBuilder(b.GetName(), fb.messages).([]*MessageBuilder)
	case *FieldBuilder:
		fb.extensions = deleteBuilder(b.GetName(), fb.extensions).([]*FieldBuilder)
	case *EnumBuilder:
		fb.enums = deleteBuilder(b.GetName(), fb.enums).([]*EnumBuilder)
	case *ServiceBuilder:
		fb.services = deleteBuilder(b.GetName(), fb.services).([]*ServiceBuilder)
	}
	delete(fb.symbols, b.GetName())
	b.setParent(nil)
}

func (fb *FileBuilder) renamedChild(b Builder, oldName string) error {
	if p, ok := b.GetParent().(*FileBuilder); !ok || p != fb {
		return nil
	}

	if err := fb.addSymbol(b); err != nil {
		return err
	}
	delete(fb.symbols, oldName)
	return nil
}

func (fb *FileBuilder) addSymbol(b Builder) error {
	if ex, ok := fb.symbols[b.GetName()]; ok {
		return fmt.Errorf("file %q already contains element (%T) named %q", fb.GetName(), ex, b.GetName())
	}
	fb.symbols[b.GetName()] = b
	return nil
}

func (fb *FileBuilder) findFullyQualifiedElement(fqn string) Builder {
	if fb.Package != "" {
		if !strings.HasPrefix(fqn, fb.Package+".") {
			return nil
		}
		fqn = fqn[len(fb.Package)+1:]
	}
	names := strings.Split(fqn, ".")
	var b Builder = fb
	for b != nil && len(names) > 0 {
		b = b.findChild(names[0])
		names = names[1:]
	}
	return b
}

// GetMessage returns the top-level message with the given name. If no such
// message exists in the file, nil is returned.
func (fb *FileBuilder) GetMessage(name string) *MessageBuilder {
	b := fb.symbols[name]
	if mb, ok := b.(*MessageBuilder); ok {
		return mb
	} else {
		return nil
	}
}

// RemoveMessage removes the top-level message with the given name. If no such
// message exists in the file, this is a no-op. This returns the file builder,
// for method chaining.
func (fb *FileBuilder) RemoveMessage(name string) *FileBuilder {
	fb.TryRemoveMessage(name)
	return fb
}

// TryRemoveMessage removes the top-level message with the given name and
// returns false if the file has no such message.
func (fb *FileBuilder) TryRemoveMessage(name string) bool {
	b := fb.symbols[name]
	if mb, ok := b.(*MessageBuilder); ok {
		fb.removeChild(mb)
		return true
	}
	return false
}

// AddMessage adds the given message to this file. If an error prevents the
// message from being added, this method panics. This returns the file builder,
// for method chaining.
func (fb *FileBuilder) AddMessage(mb *MessageBuilder) *FileBuilder {
	if err := fb.TryAddMessage(mb); err != nil {
		panic(err)
	}
	return fb
}

// TryAddMessage adds the given message to this file, returning any error that
// prevents the message from being added (such as a name collision with another
// element already added to the file).
func (fb *FileBuilder) TryAddMessage(mb *MessageBuilder) error {
	if err := fb.addSymbol(mb); err != nil {
		return err
	}
	Unlink(mb)
	mb.setParent(fb)
	fb.messages = append(fb.messages, mb)
	return nil
}

// GetExtension returns the top-level extension with the given name. If no such
// extension exists in the file, nil is returned.
func (fb *FileBuilder) GetExtension(name string) *FieldBuilder {
	b := fb.symbols[name]
	if exb, ok := b.(*FieldBuilder); ok {
		return exb
	} else {
		return nil
	}
}

// RemoveExtension removes the top-level extension with the given name. If no
// such extension exists in the file, this is a no-op. This returns the file
// builder, for method chaining.
func (fb *FileBuilder) RemoveExtension(name string) *FileBuilder {
	fb.TryRemoveExtension(name)
	return fb
}

// TryRemoveExtension removes the top-level extension with the given name and
// returns false if the file has no such extension.
func (fb *FileBuilder) TryRemoveExtension(name string) bool {
	b := fb.symbols[name]
	if exb, ok := b.(*FieldBuilder); ok {
		fb.removeChild(exb)
		return true
	}
	return false
}

// AddExtension adds the given extension to this file. If an error prevents the
// extension from being added, this method panics. This returns the file
// builder, for method chaining.
func (fb *FileBuilder) AddExtension(exb *FieldBuilder) *FileBuilder {
	if err := fb.TryAddExtension(exb); err != nil {
		panic(err)
	}
	return fb
}

// TryAddExtension adds the given extension to this file, returning any error
// that prevents the extension from being added (such as a name collision with
// another element already added to the file).
func (fb *FileBuilder) TryAddExtension(exb *FieldBuilder) error {
	if !exb.IsExtension() {
		return fmt.Errorf("field %s is not an extension", exb.GetName())
	}
	if err := fb.addSymbol(exb); err != nil {
		return err
	}
	Unlink(exb)
	exb.setParent(fb)
	fb.extensions = append(fb.extensions, exb)
	return nil
}

// GetEnum returns the top-level enum with the given name. If no such enum
// exists in the file, nil is returned.
func (fb *FileBuilder) GetEnum(name string) *EnumBuilder {
	b := fb.symbols[name]
	if eb, ok := b.(*EnumBuilder); ok {
		return eb
	} else {
		return nil
	}
}

// RemoveEnum removes the top-level enum with the given name. If no such enum
// exists in the file, this is a no-op. This returns the file builder, for
// method chaining.
func (fb *FileBuilder) RemoveEnum(name string) *FileBuilder {
	fb.TryRemoveEnum(name)
	return fb
}

// TryRemoveEnum removes the top-level enum with the given name and returns
// false if the file has no such enum.
func (fb *FileBuilder) TryRemoveEnum(name string) bool {
	b := fb.symbols[name]
	if eb, ok := b.(*EnumBuilder); ok {
		fb.removeChild(eb)
		return true
	}
	return false
}

// AddEnum adds the given enum to this file. If an error prevents the enum from
// being added, this method panics. This returns the file builder, for method
// chaining.
func (fb *FileBuilder) AddEnum(eb *EnumBuilder) *FileBuilder {
	if err := fb.TryAddEnum(eb); err != nil {
		panic(err)
	}
	return fb
}

// TryAddEnum adds the given enum to this file, returning any error that
// prevents the enum from being added (such as a name collision with another
// element already added to the file).
func (fb *FileBuilder) TryAddEnum(eb *EnumBuilder) error {
	if err := fb.addSymbol(eb); err != nil {
		return err
	}
	Unlink(eb)
	eb.setParent(fb)
	fb.enums = append(fb.enums, eb)
	return nil
}

// GetService returns the top-level service with the given name. If no such
// service exists in the file, nil is returned.
func (fb *FileBuilder) GetService(name string) *ServiceBuilder {
	b := fb.symbols[name]
	if sb, ok := b.(*ServiceBuilder); ok {
		return sb
	} else {
		return nil
	}
}

// RemoveService removes the top-level service with the given name. If no such
// service exists in the file, this is a no-op. This returns the file builder,
// for method chaining.
func (fb *FileBuilder) RemoveService(name string) *FileBuilder {
	fb.TryRemoveService(name)
	return fb
}

// TryRemoveService removes the top-level service with the given name and
// returns false if the file has no such service.
func (fb *FileBuilder) TryRemoveService(name string) bool {
	b := fb.symbols[name]
	if sb, ok := b.(*ServiceBuilder); ok {
		fb.removeChild(sb)
		return true
	}
	return false
}

// AddService adds the given service to this file. If an error prevents the
// service from being added, this method panics. This returns the file builder,
// for method chaining.
func (fb *FileBuilder) AddService(sb *ServiceBuilder) *FileBuilder {
	if err := fb.TryAddService(sb); err != nil {
		panic(err)
	}
	return fb
}

// TryAddService adds the given service to this file, returning any error that
// prevents the service from being added (such as a name collision with another
// element already added to the file).
func (fb *FileBuilder) TryAddService(sb *ServiceBuilder) error {
	if err := fb.addSymbol(sb); err != nil {
		return err
	}
	Unlink(sb)
	sb.setParent(fb)
	fb.services = append(fb.services, sb)
	return nil
}

// SetOptions sets the file options for this file and returns the file, for
// method chaining.
func (fb *FileBuilder) SetOptions(options *dpb.FileOptions) *FileBuilder {
	fb.Options = options
	return fb
}

// SetPackageName sets the name of the package for this file and returns the
// file, for method chaining.
func (fb *FileBuilder) SetPackageName(pkg string) *FileBuilder {
	fb.Package = pkg
	return fb
}

// SetProto3 sets whether this file is declared to use "proto3" syntax or not
// and returns the file, for method chaining.
func (fb *FileBuilder) SetProto3(isProto3 bool) *FileBuilder {
	fb.IsProto3 = isProto3
	return fb
}

func (fb *FileBuilder) buildProto() (*dpb.FileDescriptorProto, error) {
	name := fb.name
	if name == "" {
		name = uniqueFileName()
	}
	var syntax *string
	if fb.IsProto3 {
		syntax = proto.String("proto3")
	}
	var pkg *string
	if fb.Package != "" {
		pkg = proto.String(fb.Package)
	}

	path := make([]int32, 0, 10)
	sourceInfo := dpb.SourceCodeInfo{}
	addCommentsTo(&sourceInfo, path, &fb.comments)
	addCommentsTo(&sourceInfo, append(path, File_syntaxTag), &fb.SyntaxComments)
	addCommentsTo(&sourceInfo, append(path, File_packageTag), &fb.PackageComments)

	messages := make([]*dpb.DescriptorProto, 0, len(fb.messages))
	for _, mb := range fb.messages {
		path := append(path, File_messagesTag, int32(len(messages)))
		if md, err := mb.buildProto(path, &sourceInfo); err != nil {
			return nil, err
		} else {
			messages = append(messages, md)
		}
	}

	enums := make([]*dpb.EnumDescriptorProto, 0, len(fb.enums))
	for _, eb := range fb.enums {
		path := append(path, File_enumsTag, int32(len(enums)))
		if ed, err := eb.buildProto(path, &sourceInfo); err != nil {
			return nil, err
		} else {
			enums = append(enums, ed)
		}
	}

	extensions := make([]*dpb.FieldDescriptorProto, 0, len(fb.extensions))
	for _, exb := range fb.extensions {
		path := append(path, File_extensionsTag, int32(len(extensions)))
		if exd, err := exb.buildProto(path, &sourceInfo); err != nil {
			return nil, err
		} else {
			extensions = append(extensions, exd)
		}
	}

	services := make([]*dpb.ServiceDescriptorProto, 0, len(fb.services))
	for _, sb := range fb.services {
		path := append(path, File_servicesTag, int32(len(services)))
		if sd, err := sb.buildProto(path, &sourceInfo); err != nil {
			return nil, err
		} else {
			services = append(services, sd)
		}
	}

	return &dpb.FileDescriptorProto{
		Name:           proto.String(name),
		Package:        pkg,
		Options:        fb.Options,
		Syntax:         syntax,
		MessageType:    messages,
		EnumType:       enums,
		Extension:      extensions,
		Service:        services,
		SourceCodeInfo: &sourceInfo,
	}, nil
}

// Build constructs a file descriptor based on the contents of this file
// builder. If there are any problems constructing the descriptor, including
// resolving symbols referenced by the builder or failing to meet certain
// validation rules, an error is returned.
func (fb *FileBuilder) Build() (*desc.FileDescriptor, error) {
	return newResolver().resolveElement(fb, nil)
}

// MessageBuilder is a builder used to construct a desc.MessageDescriptor. A
// message builder can define nested messages, enums, and extensions in addition
// to defining the message's fields.
//
// Note that when building a descriptor from a MessageBuilder, not all protobuf
// validation rules are enforced. See the package documentation for more info.
type MessageBuilder struct {
	baseBuilder

	Options         *dpb.MessageOptions
	ExtensionRanges []*dpb.DescriptorProto_ExtensionRange
	ReservedRanges  []*dpb.DescriptorProto_ReservedRange
	ReservedNames   []string

	fieldsAndOneOfs  []Builder
	fieldTags        map[int32]*FieldBuilder
	nestedMessages   []*MessageBuilder
	nestedExtensions []*FieldBuilder
	nestedEnums      []*EnumBuilder
	symbols          map[string]Builder
}

// NewMessage creates a new MessageBuilder for a message with the given name.
// Since the new message has no parent element, it also has no package name
// (e.g. it is in the unnamed package, until it is assigned to a file builder
// that defines a package name).
func NewMessage(name string) *MessageBuilder {
	return &MessageBuilder{
		baseBuilder: baseBuilderWithName(name),
		fieldTags:   map[int32]*FieldBuilder{},
		symbols:     map[string]Builder{},
	}
}

// FromMessage returns a MessageBuilder that is effectively a copy of the given
// descriptor.
//
// Note that it is not just the given message that is copied but its entire
// file. So the caller can get the parent element of the returned builder and
// the result would be a builder that is effectively a copy of the message
// descriptor's parent.
//
// This means that message builders created from descriptors do not need to be
// explicitly assigned to a file in order to preserve the original message's
// package name.
func FromMessage(md *desc.MessageDescriptor) (*MessageBuilder, error) {
	if fb, err := FromFile(md.GetFile()); err != nil {
		return nil, err
	} else if mb, ok := fb.findFullyQualifiedElement(md.GetFullyQualifiedName()).(*MessageBuilder); ok {
		return mb, nil
	} else {
		return nil, fmt.Errorf("could not find message %s after converting file %q to builder", md.GetFullyQualifiedName(), md.GetFile().GetName())
	}
}

func fromMessage(md *desc.MessageDescriptor,
	localMessages map[*desc.MessageDescriptor]*MessageBuilder,
	localEnums map[*desc.EnumDescriptor]*EnumBuilder) (*MessageBuilder, error) {

	mb := NewMessage(md.GetName())
	mb.Options = md.GetMessageOptions()
	mb.ExtensionRanges = md.AsDescriptorProto().GetExtensionRange()
	mb.ReservedRanges = md.AsDescriptorProto().GetReservedRange()
	mb.ReservedNames = md.AsDescriptorProto().GetReservedName()
	setComments(&mb.comments, md.GetSourceInfo())

	localMessages[md] = mb

	oneOfs := make([]*OneOfBuilder, len(md.GetOneOfs()))
	for i, ood := range md.GetOneOfs() {
		if oob, err := fromOneOf(ood); err != nil {
			return nil, err
		} else {
			oneOfs[i] = oob
		}
	}

	for _, fld := range md.GetFields() {
		if fld.GetOneOf() != nil {
			// add one-ofs in the order of their first constituent field
			oob := oneOfs[fld.AsFieldDescriptorProto().GetOneofIndex()]
			if oob != nil {
				oneOfs[fld.AsFieldDescriptorProto().GetOneofIndex()] = nil
				if err := mb.TryAddOneOf(oob); err != nil {
					return nil, err
				}
			}
			continue
		}
		if flb, err := fromField(fld); err != nil {
			return nil, err
		} else if err := mb.TryAddField(flb); err != nil {
			return nil, err
		}
	}

	for _, nmd := range md.GetNestedMessageTypes() {
		if nmb, err := fromMessage(nmd, localMessages, localEnums); err != nil {
			return nil, err
		} else if err := mb.TryAddNestedMessage(nmb); err != nil {
			return nil, err
		}
	}
	for _, ed := range md.GetNestedEnumTypes() {
		if eb, err := fromEnum(ed, localEnums); err != nil {
			return nil, err
		} else if err := mb.TryAddNestedEnum(eb); err != nil {
			return nil, err
		}
	}
	for _, exd := range md.GetNestedExtensions() {
		if exb, err := fromField(exd); err != nil {
			return nil, err
		} else if err := mb.TryAddNestedExtension(exb); err != nil {
			return nil, err
		}
	}

	return mb, nil
}

// SetName changes this message's name, returning the message for method
// chaining. If there is an error that prevents the rename from succeeding, this
// method will panic.
func (mb *MessageBuilder) SetName(newName string) *MessageBuilder {
	if err := mb.TrySetName(newName); err != nil {
		panic(err)
	}
	return mb
}

func (mb *MessageBuilder) TrySetName(newName string) error {
	if p, ok := mb.parent.(*FieldBuilder); ok && p.fieldType.fieldType != dpb.FieldDescriptorProto_TYPE_GROUP {
		return fmt.Errorf("cannot change name of map entry %s; change name of field instead", GetFullyQualifiedName(mb))
	}
	return mb.trySetNameInternal(newName)
}

func (mb *MessageBuilder) trySetNameInternal(newName string) error {
	return mb.baseBuilder.setName(mb, newName)
}

func (mb *MessageBuilder) setNameInternal(newName string) {
	if err := mb.trySetNameInternal(newName); err != nil {
		panic(err)
	}
}

func (mb *MessageBuilder) SetComments(c Comments) *MessageBuilder {
	mb.comments = c
	return mb
}

func (mb *MessageBuilder) GetChildren() []Builder {
	var ch []Builder
	for _, b := range mb.fieldsAndOneOfs {
		ch = append(ch, b)
	}
	for _, nmb := range mb.nestedMessages {
		ch = append(ch, nmb)
	}
	for _, exb := range mb.nestedExtensions {
		ch = append(ch, exb)
	}
	for _, eb := range mb.nestedEnums {
		ch = append(ch, eb)
	}
	return ch
}

func (mb *MessageBuilder) findChild(name string) Builder {
	return mb.symbols[name]
}

func (mb *MessageBuilder) removeChild(b Builder) {
	if p, ok := b.GetParent().(*MessageBuilder); !ok || p != mb {
		return
	}

	switch b := b.(type) {
	case *FieldBuilder:
		if b.IsExtension() {
			mb.nestedExtensions = deleteBuilder(b.GetName(), mb.nestedExtensions).([]*FieldBuilder)
		} else {
			mb.fieldsAndOneOfs = deleteBuilder(b.GetName(), mb.fieldsAndOneOfs).([]Builder)
			delete(mb.fieldTags, b.GetNumber())
			if b.msgType != nil {
				delete(mb.symbols, b.msgType.GetName())
			}
		}
	case *OneOfBuilder:
		mb.fieldsAndOneOfs = deleteBuilder(b.GetName(), mb.fieldsAndOneOfs).([]Builder)
		for _, flb := range b.choices {
			delete(mb.symbols, flb.GetName())
			delete(mb.fieldTags, flb.GetNumber())
		}
	case *MessageBuilder:
		mb.nestedMessages = deleteBuilder(b.GetName(), mb.nestedMessages).([]*MessageBuilder)
	case *EnumBuilder:
		mb.nestedEnums = deleteBuilder(b.GetName(), mb.nestedEnums).([]*EnumBuilder)
	}
	delete(mb.symbols, b.GetName())
	b.setParent(nil)
}

func (mb *MessageBuilder) renamedChild(b Builder, oldName string) error {
	if p, ok := b.GetParent().(*MessageBuilder); !ok || p != mb {
		return nil
	}

	if err := mb.addSymbol(b); err != nil {
		return err
	}
	delete(mb.symbols, oldName)
	return nil
}

func (mb *MessageBuilder) addSymbol(b Builder) error {
	if ex, ok := mb.symbols[b.GetName()]; ok {
		return fmt.Errorf("message %s already contains element (%T) named %q", GetFullyQualifiedName(mb), ex, b.GetName())
	}
	mb.symbols[b.GetName()] = b
	return nil
}

func (mb *MessageBuilder) addTag(flb *FieldBuilder) error {
	if flb.number == 0 {
		return nil
	}
	if ex, ok := mb.fieldTags[flb.GetNumber()]; ok {
		return fmt.Errorf("message %s already contains field with tag %d: %s", GetFullyQualifiedName(mb), flb.GetNumber(), ex.GetName())
	}
	mb.fieldTags[flb.GetNumber()] = flb
	return nil
}

func (mb *MessageBuilder) registerField(flb *FieldBuilder) error {
	if err := mb.addSymbol(flb); err != nil {
		return err
	}
	if err := mb.addTag(flb); err != nil {
		delete(mb.symbols, flb.GetName())
		return err
	}
	if flb.msgType != nil {
		if err := mb.addSymbol(flb.msgType); err != nil {
			delete(mb.symbols, flb.GetName())
			delete(mb.fieldTags, flb.GetNumber())
			return err
		}
	}
	return nil
}

// GetField returns the field with the given name. If no such field exists in
// the message, nil is returned. The field does not have to be an immediate
// child of this message but could instead be an indirect child via a one-of.
func (mb *MessageBuilder) GetField(name string) *FieldBuilder {
	b := mb.symbols[name]
	if flb, ok := b.(*FieldBuilder); ok && !flb.IsExtension() {
		return flb
	} else {
		return nil
	}
}

// RemoveField removes the field with the given name. If no such field exists in
// the message, this is a no-op. If the field is part of a one-of, the one-of
// remains assigned to this message and the field is removed from it. This
// returns the message builder, for method chaining.
func (mb *MessageBuilder) RemoveField(name string) *MessageBuilder {
	mb.TryRemoveField(name)
	return mb
}

// TryRemoveField removes the field with the given name and returns false if the
// message has no such field. If the field is part of a one-of, the one-of
// remains assigned to this message and the field is removed from it.
func (mb *MessageBuilder) TryRemoveField(name string) bool {
	b := mb.symbols[name]
	if flb, ok := b.(*FieldBuilder); ok && !flb.IsExtension() {
		// parent could be mb, but could also be a one-of
		flb.GetParent().removeChild(flb)
		return true
	}
	return false
}

// AddField adds the given field to this message. If an error prevents the field
// from being added, this method panics. If the given field is an extension,
// this method panics. This returns the message builder, for method chaining.
func (mb *MessageBuilder) AddField(flb *FieldBuilder) *MessageBuilder {
	if err := mb.TryAddField(flb); err != nil {
		panic(err)
	}
	return mb
}

// TryAddField adds the given field to this message, returning any error that
// prevents the field from being added (such as a name collision with another
// element already added to the message). An error is returned if the given
// field is an extension field.
func (mb *MessageBuilder) TryAddField(flb *FieldBuilder) error {
	if flb.IsExtension() {
		return fmt.Errorf("field %s is an extension, not a regular field", flb.GetName())
	}
	// If we are moving field from a one-of that belongs to this message
	// directly to this message, we have to use different order of operations
	// to prevent failure (otherwise, it looks like it's being added twice).
	// (We do similar if moving the other direction, from message to a one-of
	// that is already assigned to same message.)
	needToUnlinkFirst := mb.isPresentButNotChild(flb)
	if needToUnlinkFirst {
		Unlink(flb)
		mb.registerField(flb)
	} else {
		if err := mb.registerField(flb); err != nil {
			return err
		}
		Unlink(flb)
	}
	flb.setParent(mb)
	mb.fieldsAndOneOfs = append(mb.fieldsAndOneOfs, flb)
	return nil
}

// GetOneOf returns the one-of with the given name. If no such one-of exists in
// the message, nil is returned.
func (mb *MessageBuilder) GetOneOf(name string) *OneOfBuilder {
	b := mb.symbols[name]
	if oob, ok := b.(*OneOfBuilder); ok {
		return oob
	} else {
		return nil
	}
}

// RemoveOneOf removes the one-of with the given name. If no such one-of exists
// in the message, this is a no-op. This returns the message builder, for method
// chaining.
func (mb *MessageBuilder) RemoveOneOf(name string) *MessageBuilder {
	mb.TryRemoveOneOf(name)
	return mb
}

// TryRemoveOneOf removes the one-of with the given name and returns false if
// the message has no such one-of.
func (mb *MessageBuilder) TryRemoveOneOf(name string) bool {
	b := mb.symbols[name]
	if oob, ok := b.(*OneOfBuilder); ok {
		mb.removeChild(oob)
		return true
	}
	return false
}

// AddOneOf adds the given one-of to this message. If an error prevents the
// one-of from being added, this method panics. This returns the message
// builder, for method chaining.
func (mb *MessageBuilder) AddOneOf(oob *OneOfBuilder) *MessageBuilder {
	if err := mb.TryAddOneOf(oob); err != nil {
		panic(err)
	}
	return mb
}

// TryAddOneOf adds the given one-of to this message, returning any error that
// prevents the one-of from being added (such as a name collision with another
// element already added to the message).
func (mb *MessageBuilder) TryAddOneOf(oob *OneOfBuilder) error {
	if err := mb.addSymbol(oob); err != nil {
		return err
	}
	// add nested fields to symbol and tag map
	for i, flb := range oob.choices {
		if err := mb.registerField(flb); err != nil {
			// must undo all additions we've made so far
			delete(mb.symbols, oob.GetName())
			for i > 1 {
				i--
				flb := oob.choices[i]
				delete(mb.symbols, flb.GetName())
				delete(mb.fieldTags, flb.GetNumber())
			}
			return err
		}
	}
	Unlink(oob)
	oob.setParent(mb)
	mb.fieldsAndOneOfs = append(mb.fieldsAndOneOfs, oob)
	return nil
}

// GetNestedMessage returns the nested message with the given name. If no such
// message exists, nil is returned. The named message must be in this message's
// scope. If the message is nested more deeply, this will return nil. This means
// the message must be a direct child of this message or a child of one of this
// message's fields (e.g. the group type for a group field or a map entry for a
// map field).
func (mb *MessageBuilder) GetNestedMessage(name string) *MessageBuilder {
	b := mb.symbols[name]
	if nmb, ok := b.(*MessageBuilder); ok {
		return nmb
	} else {
		return nil
	}
}

// RemoveNestedMessage removes the nested message with the given name. If no
// such message exists, this is a no-op. This returns the message builder, for
// method chaining.
func (mb *MessageBuilder) RemoveNestedMessage(name string) *MessageBuilder {
	mb.TryRemoveNestedMessage(name)
	return mb
}

// TryRemoveNestedMessage removes the nested message with the given name and
// returns false if this message has no nested message with that name. If the
// named message is a child of a field (e.g. the group type for a group field or
// the map entry for a map field), it is removed from that field and thus
// removed from this message's scope.
func (mb *MessageBuilder) TryRemoveNestedMessage(name string) bool {
	b := mb.symbols[name]
	if nmb, ok := b.(*MessageBuilder); ok {
		// parent could be mb, but could also be a field (if the message
		// is the field's group or map entry type)
		nmb.GetParent().removeChild(nmb)
		return true
	}
	return false
}

// AddNestedMessage adds the given message as nested child of this message. If
// an error prevents the message from being added, this method panics. This
// returns the message builder, for method chaining.
func (mb *MessageBuilder) AddNestedMessage(nmb *MessageBuilder) *MessageBuilder {
	if err := mb.TryAddNestedMessage(nmb); err != nil {
		panic(err)
	}
	return mb
}

// TryAddNestedMessage adds the given message as a nested child of this message,
// returning any error that prevents the message from being added (such as a
// name collision with another element already added to the message).
func (mb *MessageBuilder) TryAddNestedMessage(nmb *MessageBuilder) error {
	// If we are moving nested message from field (map entry or group type)
	// directly to this message, we have to use different order of operations
	// to prevent failure (otherwise, it looks like it's being added twice).
	// (We don't need to do similar for the other direction, because that isn't
	// possible: you can't add messages to a field, they can only be constructed
	// that way using NewGroupField or NewMapField.)
	needToUnlinkFirst := mb.isPresentButNotChild(nmb)
	if needToUnlinkFirst {
		Unlink(nmb)
		mb.addSymbol(nmb)
	} else {
		if err := mb.addSymbol(nmb); err != nil {
			return err
		}
		Unlink(mb)
	}
	nmb.setParent(mb)
	mb.nestedMessages = append(mb.nestedMessages, nmb)
	return nil
}

func (mb *MessageBuilder) isPresentButNotChild(b Builder) bool {
	if p, ok := b.GetParent().(*MessageBuilder); ok && p == mb {
		// it's a child
		return false
	}
	return mb.symbols[b.GetName()] == b
}

func (mb *MessageBuilder) GetNestedExtension(name string) *FieldBuilder {
	b := mb.symbols[name]
	if exb, ok := b.(*FieldBuilder); ok && exb.IsExtension() {
		return exb
	} else {
		return nil
	}
}

func (mb *MessageBuilder) RemoveNestedExtension(name string) *MessageBuilder {
	mb.TryRemoveNestedExtension(name)
	return mb
}

func (mb *MessageBuilder) TryRemoveNestedExtension(name string) bool {
	b := mb.symbols[name]
	if exb, ok := b.(*FieldBuilder); ok && exb.IsExtension() {
		mb.removeChild(exb)
		return true
	}
	return false
}

func (mb *MessageBuilder) AddNestedExtension(exb *FieldBuilder) *MessageBuilder {
	if err := mb.TryAddNestedExtension(exb); err != nil {
		panic(err)
	}
	return mb
}

func (mb *MessageBuilder) TryAddNestedExtension(exb *FieldBuilder) error {
	if !exb.IsExtension() {
		return fmt.Errorf("field %s is not an extension", exb.GetName())
	}
	if err := mb.addSymbol(exb); err != nil {
		return err
	}
	Unlink(exb)
	exb.setParent(mb)
	mb.nestedExtensions = append(mb.nestedExtensions, exb)
	return nil
}

func (mb *MessageBuilder) GetNestedEnum(name string) *EnumBuilder {
	b := mb.symbols[name]
	if eb, ok := b.(*EnumBuilder); ok {
		return eb
	} else {
		return nil
	}
}

func (mb *MessageBuilder) RemoveNestedEnum(name string) *MessageBuilder {
	mb.TryRemoveNestedEnum(name)
	return mb
}

func (mb *MessageBuilder) TryRemoveNestedEnum(name string) bool {
	b := mb.symbols[name]
	if eb, ok := b.(*EnumBuilder); ok {
		mb.removeChild(eb)
		return true
	}
	return false
}

func (mb *MessageBuilder) AddNestedEnum(eb *EnumBuilder) *MessageBuilder {
	if err := mb.TryAddNestedEnum(eb); err != nil {
		panic(err)
	}
	return mb
}

func (mb *MessageBuilder) TryAddNestedEnum(eb *EnumBuilder) error {
	if err := mb.addSymbol(eb); err != nil {
		return err
	}
	Unlink(eb)
	eb.setParent(mb)
	mb.nestedEnums = append(mb.nestedEnums, eb)
	return nil
}

func (mb *MessageBuilder) SetOptions(options *dpb.MessageOptions) *MessageBuilder {
	mb.Options = options
	return mb
}

// AddExtensionRange adds the given extension range to this message. The range
// is inclusive of both the start and end, just like defining a range in proto
// IDL source. This returns the message, for method chaining.
func (mb *MessageBuilder) AddExtensionRange(start, end int32) *MessageBuilder {
	return mb.AddExtensionRangeWithOptions(start, end, nil)
}

// AddExtensionRangeWithOptions adds the given extension range to this message.
// The range is inclusive of both the start and end, just like defining a range
// in proto IDL source. This returns the message, for method chaining.
func (mb *MessageBuilder) AddExtensionRangeWithOptions(start, end int32, options *dpb.ExtensionRangeOptions) *MessageBuilder {
	er := &dpb.DescriptorProto_ExtensionRange{
		Start:   proto.Int32(start),
		End:     proto.Int32(end + 1),
		Options: options,
	}
	mb.ExtensionRanges = append(mb.ExtensionRanges, er)
	return mb
}

// SetExtensionRanges replaces all of this message's extension ranges with the
// given slice of ranges. Unlike AddExtensionRange and unlike the way ranges are
// defined in proto IDL source, a DescriptorProto_ExtensionRange struct treats
// the end of the range as *exclusive*. So the range is inclusive of the start
// but exclusive of the end. This returns the message, for method chaining.
func (mb *MessageBuilder) SetExtensionRanges(ranges []*dpb.DescriptorProto_ExtensionRange) *MessageBuilder {
	mb.ExtensionRanges = ranges
	return mb
}

// AddReservedRange adds the given reserved range to this message. The range is
// inclusive of both the start and end, just like defining a range in proto IDL
// source. This returns the message, for method chaining.
func (mb *MessageBuilder) AddReservedRange(start, end int32) *MessageBuilder {
	rr := &dpb.DescriptorProto_ReservedRange{
		Start: proto.Int32(start),
		End:   proto.Int32(end + 1),
	}
	mb.ReservedRanges = append(mb.ReservedRanges, rr)
	return mb
}

// SetReservedRanges replaces all of this message's reserved ranges with the
// given slice of ranges. Unlike AddReservedRange and unlike the way ranges are
// defined in proto IDL source, a DescriptorProto_ReservedRange struct treats
// the end of the range as *exclusive*. So the range is inclusive of the start
// but exclusive of the end. This returns the message, for method chaining.
func (mb *MessageBuilder) SetReservedRanges(ranges []*dpb.DescriptorProto_ReservedRange) *MessageBuilder {
	mb.ReservedRanges = ranges
	return mb
}

// AddReservedName adds the given name to the list of reserved field names for
// this message. This returns the message, for method chaining.
func (mb *MessageBuilder) AddReservedName(name string) *MessageBuilder {
	mb.ReservedNames = append(mb.ReservedNames, name)
	return mb
}

// SetReservedNames replaces all of this message's reserved field names with the
// given slice of names. This returns the message, for method chaining.
func (mb *MessageBuilder) SetReservedNames(names []string) *MessageBuilder {
	mb.ReservedNames = names
	return mb
}

func (mb *MessageBuilder) buildProto(path []int32, sourceInfo *dpb.SourceCodeInfo) (*dpb.DescriptorProto, error) {
	addCommentsTo(sourceInfo, path, &mb.comments)

	var needTagsAssigned []*dpb.FieldDescriptorProto
	nestedMessages := make([]*dpb.DescriptorProto, 0, len(mb.nestedMessages))
	oneOfCount := 0
	for _, b := range mb.fieldsAndOneOfs {
		if _, ok := b.(*OneOfBuilder); ok {
			oneOfCount++
		}
	}
	fields := make([]*dpb.FieldDescriptorProto, 0, len(mb.fieldsAndOneOfs)-oneOfCount)
	oneOfs := make([]*dpb.OneofDescriptorProto, 0, oneOfCount)
	for _, b := range mb.fieldsAndOneOfs {
		if flb, ok := b.(*FieldBuilder); ok {
			fldpath := append(path, Message_fieldsTag, int32(len(fields)))
			fld, err := flb.buildProto(fldpath, sourceInfo)
			if err != nil {
				return nil, err
			}
			fields = append(fields, fld)
			if flb.number == 0 {
				needTagsAssigned = append(needTagsAssigned, fld)
			}
			if flb.msgType != nil {
				nmpath := append(path, Message_nestedMessagesTag, int32(len(nestedMessages)))
				if entry, err := flb.msgType.buildProto(nmpath, sourceInfo); err != nil {
					return nil, err
				} else {
					nestedMessages = append(nestedMessages, entry)
				}
			}
		} else {
			oopath := append(path, Message_oneOfsTag, int32(len(oneOfs)))
			oob := b.(*OneOfBuilder)
			oobIndex := len(oneOfs)
			ood, err := oob.buildProto(oopath, sourceInfo)
			if err != nil {
				return nil, err
			}
			oneOfs = append(oneOfs, ood)
			for _, flb := range oob.choices {
				path := append(path, Message_fieldsTag, int32(len(fields)))
				fld, err := flb.buildProto(path, sourceInfo)
				if err != nil {
					return nil, err
				}
				fld.OneofIndex = proto.Int32(int32(oobIndex))
				fields = append(fields, fld)
				if flb.number == 0 {
					needTagsAssigned = append(needTagsAssigned, fld)
				}
			}
		}
	}

	if len(needTagsAssigned) > 0 {
		tags := make([]int, len(fields)-len(needTagsAssigned))
		for i, fld := range fields {
			tag := fld.GetNumber()
			if tag != 0 {
				tags[i] = int(tag)
			}
		}
		sort.Ints(tags)
		t := 1
		for len(needTagsAssigned) > 0 {
			for len(tags) > 0 && t == tags[0] {
				t++
				tags = tags[1:]
			}
			needTagsAssigned[0].Number = proto.Int32(int32(t))
			needTagsAssigned = needTagsAssigned[1:]
			t++
		}
	}

	for _, nmb := range mb.nestedMessages {
		path := append(path, Message_nestedMessagesTag, int32(len(nestedMessages)))
		if nmd, err := nmb.buildProto(path, sourceInfo); err != nil {
			return nil, err
		} else {
			nestedMessages = append(nestedMessages, nmd)
		}
	}

	nestedExtensions := make([]*dpb.FieldDescriptorProto, 0, len(mb.nestedExtensions))
	for _, exb := range mb.nestedExtensions {
		path := append(path, Message_extensionsTag, int32(len(nestedExtensions)))
		if exd, err := exb.buildProto(path, sourceInfo); err != nil {
			return nil, err
		} else {
			nestedExtensions = append(nestedExtensions, exd)
		}
	}

	nestedEnums := make([]*dpb.EnumDescriptorProto, 0, len(mb.nestedEnums))
	for _, eb := range mb.nestedEnums {
		path := append(path, Message_enumsTag, int32(len(nestedEnums)))
		if ed, err := eb.buildProto(path, sourceInfo); err != nil {
			return nil, err
		} else {
			nestedEnums = append(nestedEnums, ed)
		}
	}

	return &dpb.DescriptorProto{
		Name:           proto.String(mb.name),
		Options:        mb.Options,
		Field:          fields,
		OneofDecl:      oneOfs,
		NestedType:     nestedMessages,
		EnumType:       nestedEnums,
		Extension:      nestedExtensions,
		ExtensionRange: mb.ExtensionRanges,
		ReservedName:   mb.ReservedNames,
		ReservedRange:  mb.ReservedRanges,
	}, nil
}

// Build constructs a message descriptor based on the contents of this message
// builder. If there are any problems constructing the descriptor, including
// resolving symbols referenced by the builder or failing to meet certain
// validation rules, an error is returned.
func (mb *MessageBuilder) Build() (*desc.MessageDescriptor, error) {
	d, err := doBuild(mb)
	if err != nil {
		return nil, err
	}
	return d.(*desc.MessageDescriptor), nil
}

type FieldBuilder struct {
	baseBuilder
	number int32

	// msgType is populated for fields that have a "private" message type that
	// isn't expected to be referenced elsewhere. This happens for map fields,
	// where the private message type represents the map entry, and for group
	// fields.
	msgType   *MessageBuilder
	fieldType *FieldType

	Options  *dpb.FieldOptions
	Label    dpb.FieldDescriptorProto_Label
	Default  string
	JsonName string

	foreignExtendee *desc.MessageDescriptor
	localExtendee   *MessageBuilder
}

func NewField(name string, typ *FieldType) *FieldBuilder {
	flb := &FieldBuilder{
		baseBuilder: baseBuilderWithName(name),
		fieldType:   typ,
	}
	return flb
}

func NewMapField(name string, keyTyp, valTyp *FieldType) *FieldBuilder {
	switch keyTyp.fieldType {
	case dpb.FieldDescriptorProto_TYPE_BOOL,
		dpb.FieldDescriptorProto_TYPE_STRING,
		dpb.FieldDescriptorProto_TYPE_INT32, dpb.FieldDescriptorProto_TYPE_INT64,
		dpb.FieldDescriptorProto_TYPE_SINT32, dpb.FieldDescriptorProto_TYPE_SINT64,
		dpb.FieldDescriptorProto_TYPE_UINT32, dpb.FieldDescriptorProto_TYPE_UINT64,
		dpb.FieldDescriptorProto_TYPE_FIXED32, dpb.FieldDescriptorProto_TYPE_FIXED64,
		dpb.FieldDescriptorProto_TYPE_SFIXED32, dpb.FieldDescriptorProto_TYPE_SFIXED64:
		// allowed
	default:
		panic(fmt.Sprintf("Map types cannot have keys of type %v", keyTyp.fieldType))
	}
	if valTyp.fieldType == dpb.FieldDescriptorProto_TYPE_GROUP {
		panic(fmt.Sprintf("Map types cannot have values of type %v", valTyp.fieldType))
	}
	entryMsg := NewMessage(entryTypeName(name))
	keyFlb := NewField("key", keyTyp)
	keyFlb.number = 1
	valFlb := NewField("value", valTyp)
	valFlb.number = 2
	entryMsg.AddField(keyFlb)
	entryMsg.AddField(valFlb)
	entryMsg.Options = &dpb.MessageOptions{MapEntry: proto.Bool(true)}

	flb := NewField(name, FieldTypeMessage(entryMsg)).
		SetLabel(dpb.FieldDescriptorProto_LABEL_REPEATED)
	flb.msgType = entryMsg
	entryMsg.setParent(flb)
	return flb
}

func NewGroupField(mb *MessageBuilder) *FieldBuilder {
	if !unicode.IsUpper(rune(mb.name[0])) {
		panic(fmt.Sprintf("group name %s must start with a capital letter", mb.name))
	}
	Unlink(mb)

	ft := &FieldType{
		fieldType:    dpb.FieldDescriptorProto_TYPE_GROUP,
		localMsgType: mb,
	}
	fieldName := strings.ToLower(mb.GetName())
	flb := NewField(fieldName, ft)
	flb.msgType = mb
	mb.setParent(flb)
	return flb
}

func NewExtension(name string, tag int32, typ *FieldType, extendee *MessageBuilder) *FieldBuilder {
	if extendee == nil {
		panic("extendee cannot be nil")
	}
	flb := NewField(name, typ).SetNumber(tag)
	flb.localExtendee = extendee
	return flb
}

func NewExtensionImported(name string, tag int32, typ *FieldType, extendee *desc.MessageDescriptor) *FieldBuilder {
	if extendee == nil {
		panic("extendee cannot be nil")
	}
	flb := NewField(name, typ).SetNumber(tag)
	flb.foreignExtendee = extendee
	return flb
}

func FromField(fld *desc.FieldDescriptor) (*FieldBuilder, error) {
	if fb, err := FromFile(fld.GetFile()); err != nil {
		return nil, err
	} else if flb, ok := fb.findFullyQualifiedElement(fld.GetFullyQualifiedName()).(*FieldBuilder); ok {
		return flb, nil
	} else {
		return nil, fmt.Errorf("could not find field %s after converting file %q to builder", fld.GetFullyQualifiedName(), fld.GetFile().GetName())
	}
}

func fromField(fld *desc.FieldDescriptor) (*FieldBuilder, error) {
	var ft *FieldType
	switch fld.GetType() {
	case dpb.FieldDescriptorProto_TYPE_GROUP:
		ft = &FieldType{fieldType: dpb.FieldDescriptorProto_TYPE_GROUP, foreignMsgType: fld.GetMessageType()}
	case dpb.FieldDescriptorProto_TYPE_MESSAGE:
		ft = FieldTypeImportedMessage(fld.GetMessageType())
	case dpb.FieldDescriptorProto_TYPE_ENUM:
		ft = FieldTypeImportedEnum(fld.GetEnumType())
	default:
		ft = FieldTypeScalar(fld.GetType())
	}
	flb := NewField(fld.GetName(), ft)
	flb.Options = fld.GetFieldOptions()
	flb.Label = fld.GetLabel()
	flb.Default = fld.AsFieldDescriptorProto().GetDefaultValue()
	flb.JsonName = fld.GetJSONName()
	setComments(&flb.comments, fld.GetSourceInfo())

	if fld.IsExtension() {
		flb.foreignExtendee = fld.GetOwner()
	}
	if err := flb.TrySetNumber(fld.GetNumber()); err != nil {
		return nil, err
	}
	return flb, nil
}

func (flb *FieldBuilder) SetName(newName string) *FieldBuilder {
	if err := flb.TrySetName(newName); err != nil {
		panic(err)
	}
	return flb
}

func (flb *FieldBuilder) TrySetName(newName string) error {
	var oldMsgName string
	if flb.msgType != nil {
		if flb.fieldType.fieldType == dpb.FieldDescriptorProto_TYPE_GROUP {
			return fmt.Errorf("cannot change name of group field %s; change name of group instead", GetFullyQualifiedName(flb))
		} else {
			oldMsgName = flb.msgType.name
			msgName := entryTypeName(newName)
			if err := flb.msgType.trySetNameInternal(msgName); err != nil {
				return err
			}
		}
	}
	if err := flb.baseBuilder.setName(flb, newName); err != nil {
		// undo change to map entry name
		if flb.msgType != nil && flb.fieldType.fieldType != dpb.FieldDescriptorProto_TYPE_GROUP {
			flb.msgType.setNameInternal(oldMsgName)
		}
		return err
	}
	return nil
}

func (flb *FieldBuilder) trySetNameInternal(newName string) error {
	return flb.baseBuilder.setName(flb, newName)
}

func (flb *FieldBuilder) setNameInternal(newName string) {
	if err := flb.trySetNameInternal(newName); err != nil {
		panic(err)
	}
}

func (flb *FieldBuilder) SetComments(c Comments) *FieldBuilder {
	flb.comments = c
	return flb
}

func (flb *FieldBuilder) setParent(newParent Builder) {
	flb.baseBuilder.setParent(newParent)
}

func (flb *FieldBuilder) GetChildren() []Builder {
	if flb.msgType != nil {
		return []Builder{flb.msgType}
	}
	return nil
}

func (flb *FieldBuilder) findChild(name string) Builder {
	if flb.msgType != nil && flb.msgType.name == name {
		return flb.msgType
	}
	return nil
}

func (flb *FieldBuilder) removeChild(b Builder) {
	if mb, ok := b.(*MessageBuilder); ok && mb == flb.msgType {
		flb.msgType = nil
		if p, ok := flb.parent.(*MessageBuilder); ok {
			delete(p.symbols, mb.GetName())
		}
	}
}

func (flb *FieldBuilder) renamedChild(b Builder, oldName string) error {
	if flb.msgType != nil {
		var oldFieldName string
		if flb.fieldType.fieldType == dpb.FieldDescriptorProto_TYPE_GROUP {
			if !unicode.IsUpper(rune(b.GetName()[0])) {
				return fmt.Errorf("group name %s must start with capital letter", b.GetName())
			}
			// change field name to be lower-case form of group name
			oldFieldName = flb.name
			fieldName := strings.ToLower(b.GetName())
			if err := flb.trySetNameInternal(fieldName); err != nil {
				return err
			}
		}
		if p, ok := flb.parent.(*MessageBuilder); ok {
			if err := p.addSymbol(b); err != nil {
				if flb.fieldType.fieldType == dpb.FieldDescriptorProto_TYPE_GROUP {
					// revert the field rename
					flb.setNameInternal(oldFieldName)
				}
				return err
			}
		}
	}
	return nil
}

func (flb *FieldBuilder) GetNumber() int32 {
	return flb.number
}

func (flb *FieldBuilder) SetNumber(tag int32) *FieldBuilder {
	if err := flb.TrySetNumber(tag); err != nil {
		panic(err)
	}
	return flb
}

func (flb *FieldBuilder) TrySetNumber(tag int32) error {
	if tag == flb.number {
		return nil // no change
	}
	if tag < 0 {
		return fmt.Errorf("cannot set tag number for field %s to negative value %d", GetFullyQualifiedName(flb), tag)
	}
	if tag == 0 && flb.IsExtension() {
		return fmt.Errorf("cannot set tag number for extension %s; only regular fields can be auto-assigned", GetFullyQualifiedName(flb))
	}
	if tag >= SpecialReservedStart && tag <= SpecialReservedEnd {
		return fmt.Errorf("tag for field %s cannot be in special reserved range %d-%d", GetFullyQualifiedName(flb), SpecialReservedStart, SpecialReservedEnd)
	}
	if tag > MaxTag {
		return fmt.Errorf("tag for field %s cannot be above max %d", GetFullyQualifiedName(flb), MaxTag)
	}
	oldTag := flb.number
	flb.number = tag
	switch p := flb.parent.(type) {
	case *OneOfBuilder:
		m := p.parent()
		if m != nil {
			if err := m.addTag(flb); err != nil {
				flb.number = oldTag
				return err
			}
			delete(m.fieldTags, oldTag)
		}
	case *MessageBuilder:
		if err := p.addTag(flb); err != nil {
			flb.number = oldTag
			return err
		}
		delete(p.fieldTags, oldTag)
	}
	return nil
}

func (flb *FieldBuilder) SetOptions(options *dpb.FieldOptions) *FieldBuilder {
	flb.Options = options
	return flb
}

func (flb *FieldBuilder) SetLabel(lbl dpb.FieldDescriptorProto_Label) *FieldBuilder {
	flb.Label = lbl
	return flb
}

func (flb *FieldBuilder) SetRepeated() *FieldBuilder {
	return flb.SetLabel(dpb.FieldDescriptorProto_LABEL_REPEATED)
}

func (flb *FieldBuilder) SetRequired() *FieldBuilder {
	return flb.SetLabel(dpb.FieldDescriptorProto_LABEL_REQUIRED)
}

func (flb *FieldBuilder) SetOptional() *FieldBuilder {
	return flb.SetLabel(dpb.FieldDescriptorProto_LABEL_OPTIONAL)
}

func (flb *FieldBuilder) IsRepeated() bool {
	return flb.Label == dpb.FieldDescriptorProto_LABEL_REPEATED
}

func (flb *FieldBuilder) IsRequired() bool {
	return flb.Label == dpb.FieldDescriptorProto_LABEL_REQUIRED
}

func (flb *FieldBuilder) IsMap() bool {
	return flb.IsRepeated() &&
		flb.msgType != nil &&
		flb.fieldType.fieldType != dpb.FieldDescriptorProto_TYPE_GROUP &&
		flb.msgType.Options != nil &&
		flb.msgType.Options.GetMapEntry()
}

func (flb *FieldBuilder) GetType() *FieldType {
	return flb.fieldType
}

func (flb *FieldBuilder) SetType(ft *FieldType) *FieldBuilder {
	flb.fieldType = ft
	if flb.msgType != nil && flb.msgType != ft.localMsgType {
		Unlink(flb.msgType)
	}
	return flb
}

func (flb *FieldBuilder) SetDefaultValue(defValue string) *FieldBuilder {
	flb.Default = defValue
	return flb
}

func (flb *FieldBuilder) SetJsonName(jsonName string) *FieldBuilder {
	flb.JsonName = jsonName
	return flb
}

func (flb *FieldBuilder) IsExtension() bool {
	return flb.localExtendee != nil || flb.foreignExtendee != nil
}

func (flb *FieldBuilder) GetExtendeeTypeName() string {
	if flb.foreignExtendee != nil {
		return flb.foreignExtendee.GetFullyQualifiedName()
	} else if flb.localExtendee != nil {
		return GetFullyQualifiedName(flb.localExtendee)
	} else {
		return ""
	}
}

func (flb *FieldBuilder) buildProto(path []int32, sourceInfo *dpb.SourceCodeInfo) (*dpb.FieldDescriptorProto, error) {
	addCommentsTo(sourceInfo, path, &flb.comments)

	var lbl *dpb.FieldDescriptorProto_Label
	if int32(flb.Label) != 0 {
		lbl = flb.Label.Enum()
	}
	var typeName *string
	tn := flb.fieldType.GetTypeName()
	if tn != "" {
		typeName = proto.String("." + tn)
	}
	var extendee *string
	if flb.IsExtension() {
		extendee = proto.String("." + flb.GetExtendeeTypeName())
	}
	jsName := flb.JsonName
	if jsName == "" {
		jsName = JsonName(flb.name)
	}
	var def *string
	if flb.Default != "" {
		def = proto.String(flb.Default)
	}

	return &dpb.FieldDescriptorProto{
		Name:         proto.String(flb.name),
		Number:       proto.Int32(flb.number),
		Options:      flb.Options,
		Label:        lbl,
		Type:         flb.fieldType.fieldType.Enum(),
		TypeName:     typeName,
		JsonName:     proto.String(jsName),
		DefaultValue: def,
		Extendee:     extendee,
	}, nil
}

func (flb *FieldBuilder) Build() (*desc.FieldDescriptor, error) {
	d, err := doBuild(flb)
	if err != nil {
		return nil, err
	}
	return d.(*desc.FieldDescriptor), nil
}

type OneOfBuilder struct {
	baseBuilder

	Options *dpb.OneofOptions

	choices []*FieldBuilder
	symbols map[string]*FieldBuilder
}

func NewOneOf(name string) *OneOfBuilder {
	return &OneOfBuilder{
		baseBuilder: baseBuilderWithName(name),
		symbols:     map[string]*FieldBuilder{},
	}
}

func FromOneOf(ood *desc.OneOfDescriptor) (*OneOfBuilder, error) {
	if fb, err := FromFile(ood.GetFile()); err != nil {
		return nil, err
	} else if oob, ok := fb.findFullyQualifiedElement(ood.GetFullyQualifiedName()).(*OneOfBuilder); ok {
		return oob, nil
	} else {
		return nil, fmt.Errorf("could not find one-of %s after converting file %q to builder", ood.GetFullyQualifiedName(), ood.GetFile().GetName())
	}
}

func fromOneOf(ood *desc.OneOfDescriptor) (*OneOfBuilder, error) {
	oob := NewOneOf(ood.GetName())
	oob.Options = ood.GetOneOfOptions()
	setComments(&oob.comments, ood.GetSourceInfo())

	for _, fld := range ood.GetChoices() {
		if flb, err := fromField(fld); err != nil {
			return nil, err
		} else if err := oob.TryAddChoice(flb); err != nil {
			return nil, err
		}
	}

	return oob, nil
}

func (oob *OneOfBuilder) SetName(newName string) *OneOfBuilder {
	if err := oob.TrySetName(newName); err != nil {
		panic(err)
	}
	return oob
}

func (oob *OneOfBuilder) TrySetName(newName string) error {
	return oob.baseBuilder.setName(oob, newName)
}

func (oob *OneOfBuilder) SetComments(c Comments) *OneOfBuilder {
	oob.comments = c
	return oob
}

func (oob *OneOfBuilder) GetChildren() []Builder {
	var ch []Builder
	for _, evb := range oob.choices {
		ch = append(ch, evb)
	}
	return ch
}

func (oob *OneOfBuilder) parent() *MessageBuilder {
	if oob.baseBuilder.parent == nil {
		return nil
	}
	return oob.baseBuilder.parent.(*MessageBuilder)
}

func (oob *OneOfBuilder) findChild(name string) Builder {
	// in terms of finding a child by qualified name, fields in the
	// one-of are considered children of the message, not the one-of
	return nil
}

func (oob *OneOfBuilder) removeChild(b Builder) {
	if p, ok := b.GetParent().(*OneOfBuilder); !ok || p != oob {
		return
	}

	if oob.parent() != nil {
		// remove from message's name and tag maps
		flb := b.(*FieldBuilder)
		delete(oob.parent().fieldTags, flb.GetNumber())
		delete(oob.parent().symbols, flb.GetName())
	}

	oob.choices = deleteBuilder(b.GetName(), oob.choices).([]*FieldBuilder)
	delete(oob.symbols, b.GetName())
	b.setParent(nil)
}

func (oob *OneOfBuilder) renamedChild(b Builder, oldName string) error {
	if p, ok := b.GetParent().(*OneOfBuilder); !ok || p != oob {
		return nil
	}

	if err := oob.addSymbol(b.(*FieldBuilder)); err != nil {
		return err
	}

	// update message's name map (to make sure new field name doesn't
	// collide with other kinds of elements in the message)
	if oob.parent() != nil {
		if err := oob.parent().addSymbol(b); err != nil {
			delete(oob.symbols, b.GetName())
			return err
		}
		delete(oob.parent().symbols, oldName)
	}

	delete(oob.symbols, oldName)
	return nil
}

func (oob *OneOfBuilder) addSymbol(b *FieldBuilder) error {
	if _, ok := oob.symbols[b.GetName()]; ok {
		return fmt.Errorf("one-of %s already contains field named %q", GetFullyQualifiedName(oob), b.GetName())
	}
	oob.symbols[b.GetName()] = b
	return nil
}

func (oob *OneOfBuilder) GetChoice(name string) *FieldBuilder {
	return oob.symbols[name]
}

func (oob *OneOfBuilder) RemoveChoice(name string) *OneOfBuilder {
	oob.TryRemoveChoice(name)
	return oob
}

func (oob *OneOfBuilder) TryRemoveChoice(name string) bool {
	if flb, ok := oob.symbols[name]; ok {
		oob.removeChild(flb)
		return true
	}
	return false
}

func (oob *OneOfBuilder) AddChoice(flb *FieldBuilder) *OneOfBuilder {
	if err := oob.TryAddChoice(flb); err != nil {
		panic(err)
	}
	return oob
}

func (oob *OneOfBuilder) TryAddChoice(flb *FieldBuilder) error {
	if flb.IsExtension() {
		return fmt.Errorf("field %s is an extension, not a regular field", flb.GetName())
	}
	if flb.msgType != nil {
		return fmt.Errorf("cannot add a group or map field %q to one-of %s", flb.name, GetFullyQualifiedName(oob))
	}
	if flb.IsRepeated() || flb.IsRequired() {
		return fmt.Errorf("fields in a one-of must be optional, %s is %v", flb.name, flb.Label)
	}
	if err := oob.addSymbol(flb); err != nil {
		return err
	}
	mb := oob.parent()
	if mb != nil {
		// If we are moving field from a message to a one-of that belongs to the
		// same message, we have to use different order of operations to prevent
		// failure (otherwise, it looks like it's being added twice).
		// (We do similar if moving the other direction, from the one-of into
		// the message to which one-of belongs.)
		needToUnlinkFirst := mb.isPresentButNotChild(flb)
		if needToUnlinkFirst {
			Unlink(flb)
			mb.registerField(flb)
		} else {
			if err := mb.registerField(flb); err != nil {
				delete(oob.symbols, flb.GetName())
				return err
			}
			Unlink(flb)
		}
	}
	flb.setParent(oob)
	oob.choices = append(oob.choices, flb)
	return nil
}

func (oob *OneOfBuilder) SetOptions(options *dpb.OneofOptions) *OneOfBuilder {
	oob.Options = options
	return oob
}

func (oob *OneOfBuilder) buildProto(path []int32, sourceInfo *dpb.SourceCodeInfo) (*dpb.OneofDescriptorProto, error) {
	addCommentsTo(sourceInfo, path, &oob.comments)

	for _, flb := range oob.choices {
		if flb.IsRepeated() || flb.IsRequired() {
			return nil, fmt.Errorf("fields in a one-of must be optional, %s is %v", GetFullyQualifiedName(flb), flb.Label)
		}
	}

	return &dpb.OneofDescriptorProto{
		Name:    proto.String(oob.name),
		Options: oob.Options,
	}, nil
}

func (oob *OneOfBuilder) Build() (*desc.OneOfDescriptor, error) {
	d, err := doBuild(oob)
	if err != nil {
		return nil, err
	}
	return d.(*desc.OneOfDescriptor), nil
}

type EnumBuilder struct {
	baseBuilder

	Options *dpb.EnumOptions

	values  []*EnumValueBuilder
	symbols map[string]*EnumValueBuilder
}

func NewEnum(name string) *EnumBuilder {
	return &EnumBuilder{
		baseBuilder: baseBuilderWithName(name),
		symbols:     map[string]*EnumValueBuilder{},
	}
}

func FromEnum(ed *desc.EnumDescriptor) (*EnumBuilder, error) {
	if fb, err := FromFile(ed.GetFile()); err != nil {
		return nil, err
	} else if eb, ok := fb.findFullyQualifiedElement(ed.GetFullyQualifiedName()).(*EnumBuilder); ok {
		return eb, nil
	} else {
		return nil, fmt.Errorf("could not find enum %s after converting file %q to builder", ed.GetFullyQualifiedName(), ed.GetFile().GetName())
	}
}

func fromEnum(ed *desc.EnumDescriptor, localEnums map[*desc.EnumDescriptor]*EnumBuilder) (*EnumBuilder, error) {
	eb := NewEnum(ed.GetName())
	eb.Options = ed.GetEnumOptions()
	setComments(&eb.comments, ed.GetSourceInfo())

	localEnums[ed] = eb

	for _, evd := range ed.GetValues() {
		if evb, err := fromEnumValue(evd); err != nil {
			return nil, err
		} else if err := eb.TryAddValue(evb); err != nil {
			return nil, err
		}
	}

	return eb, nil
}

func (eb *EnumBuilder) SetName(newName string) *EnumBuilder {
	if err := eb.TrySetName(newName); err != nil {
		panic(err)
	}
	return eb
}

func (eb *EnumBuilder) TrySetName(newName string) error {
	return eb.baseBuilder.setName(eb, newName)
}

func (eb *EnumBuilder) SetComments(c Comments) *EnumBuilder {
	eb.comments = c
	return eb
}

func (eb *EnumBuilder) GetChildren() []Builder {
	var ch []Builder
	for _, evb := range eb.values {
		ch = append(ch, evb)
	}
	return ch
}

func (eb *EnumBuilder) findChild(name string) Builder {
	return eb.symbols[name]
}

func (eb *EnumBuilder) removeChild(b Builder) {
	if p, ok := b.GetParent().(*EnumBuilder); !ok || p != eb {
		return
	}
	eb.values = deleteBuilder(b.GetName(), eb.values).([]*EnumValueBuilder)
	delete(eb.symbols, b.GetName())
	b.setParent(nil)
}

func (eb *EnumBuilder) renamedChild(b Builder, oldName string) error {
	if p, ok := b.GetParent().(*EnumBuilder); !ok || p != eb {
		return nil
	}

	if err := eb.addSymbol(b.(*EnumValueBuilder)); err != nil {
		return err
	}
	delete(eb.symbols, oldName)
	return nil
}

func (eb *EnumBuilder) addSymbol(b *EnumValueBuilder) error {
	if _, ok := eb.symbols[b.GetName()]; ok {
		return fmt.Errorf("enum %s already contains value named %q", GetFullyQualifiedName(eb), b.GetName())
	}
	eb.symbols[b.GetName()] = b
	return nil
}

func (eb *EnumBuilder) SetOptions(options *dpb.EnumOptions) *EnumBuilder {
	eb.Options = options
	return eb
}

func (eb *EnumBuilder) GetValue(name string) *EnumValueBuilder {
	return eb.symbols[name]
}

func (eb *EnumBuilder) RemoveValue(name string) *EnumBuilder {
	eb.TryRemoveValue(name)
	return eb
}

func (eb *EnumBuilder) TryRemoveValue(name string) bool {
	if evb, ok := eb.symbols[name]; ok {
		eb.removeChild(evb)
		return true
	}
	return false
}

func (eb *EnumBuilder) AddValue(evb *EnumValueBuilder) *EnumBuilder {
	if err := eb.TryAddValue(evb); err != nil {
		panic(err)
	}
	return eb
}

func (eb *EnumBuilder) TryAddValue(evb *EnumValueBuilder) error {
	if err := eb.addSymbol(evb); err != nil {
		return err
	}
	Unlink(evb)
	evb.setParent(eb)
	eb.values = append(eb.values, evb)
	return nil
}

func (eb *EnumBuilder) buildProto(path []int32, sourceInfo *dpb.SourceCodeInfo) (*dpb.EnumDescriptorProto, error) {
	addCommentsTo(sourceInfo, path, &eb.comments)

	var needNumbersAssigned []*dpb.EnumValueDescriptorProto
	values := make([]*dpb.EnumValueDescriptorProto, 0, len(eb.values))
	for _, evb := range eb.values {
		path := append(path, Enum_valuesTag, int32(len(values)))
		evp, err := evb.buildProto(path, sourceInfo)
		if err != nil {
			return nil, err
		}
		values = append(values, evp)
		if !evb.numberSet {
			needNumbersAssigned = append(needNumbersAssigned, evp)
		}
	}

	if len(needNumbersAssigned) > 0 {
		tags := make([]int, len(values)-len(needNumbersAssigned))
		for i, ev := range values {
			tag := ev.GetNumber()
			if tag != 0 {
				tags[i] = int(tag)
			}
		}
		sort.Ints(tags)
		t := 0
		ti := sort.Search(len(tags), func(i int) bool {
			return tags[i] >= 0
		})
		if ti < len(tags) {
			tags = tags[ti:]
		}
		for len(needNumbersAssigned) > 0 {
			for len(tags) > 0 && t == tags[0] {
				t++
				tags = tags[1:]
			}
			needNumbersAssigned[0].Number = proto.Int32(int32(t))
			needNumbersAssigned = needNumbersAssigned[1:]
			t++
		}
	}

	return &dpb.EnumDescriptorProto{
		Name:    proto.String(eb.name),
		Options: eb.Options,
		Value:   values,
	}, nil
}

func (eb *EnumBuilder) Build() (*desc.EnumDescriptor, error) {
	d, err := doBuild(eb)
	if err != nil {
		return nil, err
	}
	return d.(*desc.EnumDescriptor), nil
}

type EnumValueBuilder struct {
	baseBuilder

	Number    int32
	numberSet bool
	Options   *dpb.EnumValueOptions
}

func NewEnumValue(name string) *EnumValueBuilder {
	return &EnumValueBuilder{baseBuilder: baseBuilderWithName(name)}
}

func FromEnumValue(evd *desc.EnumValueDescriptor) (*EnumValueBuilder, error) {
	if fb, err := FromFile(evd.GetFile()); err != nil {
		return nil, err
	} else if evb, ok := fb.findFullyQualifiedElement(evd.GetFullyQualifiedName()).(*EnumValueBuilder); ok {
		return evb, nil
	} else {
		return nil, fmt.Errorf("could not find enum value %s after converting file %q to builder", evd.GetFullyQualifiedName(), evd.GetFile().GetName())
	}
}

func fromEnumValue(evd *desc.EnumValueDescriptor) (*EnumValueBuilder, error) {
	evb := NewEnumValue(evd.GetName())
	evb.Options = evd.GetEnumValueOptions()
	evb.Number = evd.GetNumber()
	evb.numberSet = true
	setComments(&evb.comments, evd.GetSourceInfo())

	return evb, nil
}

func (evb *EnumValueBuilder) SetName(newName string) *EnumValueBuilder {
	if err := evb.TrySetName(newName); err != nil {
		panic(err)
	}
	return evb
}

func (evb *EnumValueBuilder) TrySetName(newName string) error {
	return evb.baseBuilder.setName(evb, newName)
}

func (evb *EnumValueBuilder) SetComments(c Comments) *EnumValueBuilder {
	evb.comments = c
	return evb
}

func (evb *EnumValueBuilder) GetChildren() []Builder {
	// enum values do not have children
	return nil
}

func (evb *EnumValueBuilder) findChild(name string) Builder {
	// enum values do not have children
	return nil
}

func (evb *EnumValueBuilder) removeChild(b Builder) {
	// enum values do not have children
}

func (evb *EnumValueBuilder) renamedChild(b Builder, oldName string) error {
	// enum values do not have children
	return nil
}

func (evb *EnumValueBuilder) SetOptions(options *dpb.EnumValueOptions) *EnumValueBuilder {
	evb.Options = options
	return evb
}

func (evb *EnumValueBuilder) SetNumber(number int32) *EnumValueBuilder {
	evb.Number = number
	evb.numberSet = true
	return evb
}

func (evb *EnumValueBuilder) buildProto(path []int32, sourceInfo *dpb.SourceCodeInfo) (*dpb.EnumValueDescriptorProto, error) {
	addCommentsTo(sourceInfo, path, &evb.comments)

	return &dpb.EnumValueDescriptorProto{
		Name:    proto.String(evb.name),
		Number:  proto.Int32(evb.Number),
		Options: evb.Options,
	}, nil
}

func (evb *EnumValueBuilder) Build() (*desc.EnumValueDescriptor, error) {
	d, err := doBuild(evb)
	if err != nil {
		return nil, err
	}
	return d.(*desc.EnumValueDescriptor), nil
}

type ServiceBuilder struct {
	baseBuilder

	Options *dpb.ServiceOptions

	methods []*MethodBuilder
	symbols map[string]*MethodBuilder
}

func NewService(name string) *ServiceBuilder {
	return &ServiceBuilder{
		baseBuilder: baseBuilderWithName(name),
		symbols:     map[string]*MethodBuilder{},
	}
}

func FromService(sd *desc.ServiceDescriptor) (*ServiceBuilder, error) {
	if fb, err := FromFile(sd.GetFile()); err != nil {
		return nil, err
	} else if sb, ok := fb.findFullyQualifiedElement(sd.GetFullyQualifiedName()).(*ServiceBuilder); ok {
		return sb, nil
	} else {
		return nil, fmt.Errorf("could not find service %s after converting file %q to builder", sd.GetFullyQualifiedName(), sd.GetFile().GetName())
	}
}

func fromService(sd *desc.ServiceDescriptor) (*ServiceBuilder, error) {
	sb := NewService(sd.GetName())
	sb.Options = sd.GetServiceOptions()
	setComments(&sb.comments, sd.GetSourceInfo())

	for _, mtd := range sd.GetMethods() {
		if mtb, err := fromMethod(mtd); err != nil {
			return nil, err
		} else if err := sb.TryAddMethod(mtb); err != nil {
			return nil, err
		}
	}

	return sb, nil
}

func (sb *ServiceBuilder) SetName(newName string) *ServiceBuilder {
	if err := sb.TrySetName(newName); err != nil {
		panic(err)
	}
	return sb
}

func (sb *ServiceBuilder) TrySetName(newName string) error {
	return sb.baseBuilder.setName(sb, newName)
}

func (sb *ServiceBuilder) SetComments(c Comments) *ServiceBuilder {
	sb.comments = c
	return sb
}

func (sb *ServiceBuilder) GetChildren() []Builder {
	var ch []Builder
	for _, mtb := range sb.methods {
		ch = append(ch, mtb)
	}
	return ch
}

func (sb *ServiceBuilder) findChild(name string) Builder {
	return sb.symbols[name]
}

func (sb *ServiceBuilder) removeChild(b Builder) {
	if p, ok := b.GetParent().(*ServiceBuilder); !ok || p != sb {
		return
	}
	sb.methods = deleteBuilder(b.GetName(), sb.methods).([]*MethodBuilder)
	delete(sb.symbols, b.GetName())
	b.setParent(nil)
}

func (sb *ServiceBuilder) renamedChild(b Builder, oldName string) error {
	if p, ok := b.GetParent().(*ServiceBuilder); !ok || p != sb {
		return nil
	}

	if err := sb.addSymbol(b.(*MethodBuilder)); err != nil {
		return err
	}
	delete(sb.symbols, oldName)
	return nil
}

func (sb *ServiceBuilder) addSymbol(b *MethodBuilder) error {
	if _, ok := sb.symbols[b.GetName()]; ok {
		return fmt.Errorf("service %s already contains method named %q", GetFullyQualifiedName(sb), b.GetName())
	}
	sb.symbols[b.GetName()] = b
	return nil
}

func (sb *ServiceBuilder) GetMethod(name string) *MethodBuilder {
	return sb.symbols[name]
}

func (sb *ServiceBuilder) RemoveMethod(name string) *ServiceBuilder {
	sb.TryRemoveMethod(name)
	return sb
}

func (sb *ServiceBuilder) TryRemoveMethod(name string) bool {
	if mtb, ok := sb.symbols[name]; ok {
		sb.removeChild(mtb)
		return true
	}
	return false
}

func (sb *ServiceBuilder) AddMethod(mtb *MethodBuilder) *ServiceBuilder {
	if err := sb.TryAddMethod(mtb); err != nil {
		panic(err)
	}
	return sb
}

func (sb *ServiceBuilder) TryAddMethod(mtb *MethodBuilder) error {
	if err := sb.addSymbol(mtb); err != nil {
		return err
	}
	Unlink(mtb)
	mtb.setParent(sb)
	sb.methods = append(sb.methods, mtb)
	return nil
}

func (sb *ServiceBuilder) SetOptions(options *dpb.ServiceOptions) *ServiceBuilder {
	sb.Options = options
	return sb
}

func (sb *ServiceBuilder) buildProto(path []int32, sourceInfo *dpb.SourceCodeInfo) (*dpb.ServiceDescriptorProto, error) {
	addCommentsTo(sourceInfo, path, &sb.comments)

	methods := make([]*dpb.MethodDescriptorProto, 0, len(sb.methods))
	for _, mtb := range sb.methods {
		path := append(path, Service_methodsTag, int32(len(methods)))
		if mtd, err := mtb.buildProto(path, sourceInfo); err != nil {
			return nil, err
		} else {
			methods = append(methods, mtd)
		}
	}

	return &dpb.ServiceDescriptorProto{
		Name:    proto.String(sb.name),
		Options: sb.Options,
		Method:  methods,
	}, nil
}

func (sb *ServiceBuilder) Build() (*desc.ServiceDescriptor, error) {
	d, err := doBuild(sb)
	if err != nil {
		return nil, err
	}
	return d.(*desc.ServiceDescriptor), nil
}

type MethodBuilder struct {
	baseBuilder

	Options  *dpb.MethodOptions
	ReqType  *RpcType
	RespType *RpcType
}

func NewMethod(name string, req, resp *RpcType) *MethodBuilder {
	return &MethodBuilder{
		baseBuilder: baseBuilderWithName(name),
		ReqType:     req,
		RespType:    resp,
	}
}

func FromMethod(mtd *desc.MethodDescriptor) (*MethodBuilder, error) {
	if fb, err := FromFile(mtd.GetFile()); err != nil {
		return nil, err
	} else if mtb, ok := fb.findFullyQualifiedElement(mtd.GetFullyQualifiedName()).(*MethodBuilder); ok {
		return mtb, nil
	} else {
		return nil, fmt.Errorf("could not find method %s after converting file %q to builder", mtd.GetFullyQualifiedName(), mtd.GetFile().GetName())
	}
}

func fromMethod(mtd *desc.MethodDescriptor) (*MethodBuilder, error) {
	req := RpcTypeImportedMessage(mtd.GetInputType(), mtd.IsClientStreaming())
	resp := RpcTypeImportedMessage(mtd.GetOutputType(), mtd.IsServerStreaming())
	mtb := NewMethod(mtd.GetName(), req, resp)
	mtb.Options = mtd.GetMethodOptions()
	setComments(&mtb.comments, mtd.GetSourceInfo())

	return mtb, nil
}

func (mtb *MethodBuilder) SetName(newName string) *MethodBuilder {
	if err := mtb.TrySetName(newName); err != nil {
		panic(err)
	}
	return mtb
}

func (mtb *MethodBuilder) TrySetName(newName string) error {
	return mtb.baseBuilder.setName(mtb, newName)
}

func (mtb *MethodBuilder) SetComments(c Comments) *MethodBuilder {
	mtb.comments = c
	return mtb
}

func (mtb *MethodBuilder) GetChildren() []Builder {
	// methods do not have children
	return nil
}

func (mtb *MethodBuilder) findChild(name string) Builder {
	// methods do not have children
	return nil
}

func (mtb *MethodBuilder) removeChild(b Builder) {
	// methods do not have children
}

func (mtb *MethodBuilder) renamedChild(b Builder, oldName string) error {
	// methods do not have children
	return nil
}

func (mtb *MethodBuilder) SetOptions(options *dpb.MethodOptions) *MethodBuilder {
	mtb.Options = options
	return mtb
}

func (mtb *MethodBuilder) SetRequestType(t *RpcType) *MethodBuilder {
	mtb.ReqType = t
	return mtb
}

func (mtb *MethodBuilder) SetResponseType(t *RpcType) *MethodBuilder {
	mtb.RespType = t
	return mtb
}

func (mtb *MethodBuilder) buildProto(path []int32, sourceInfo *dpb.SourceCodeInfo) (*dpb.MethodDescriptorProto, error) {
	addCommentsTo(sourceInfo, path, &mtb.comments)

	mtd := &dpb.MethodDescriptorProto{
		Name:       proto.String(mtb.name),
		Options:    mtb.Options,
		InputType:  proto.String("." + mtb.ReqType.GetTypeName()),
		OutputType: proto.String("." + mtb.RespType.GetTypeName()),
	}
	if mtb.ReqType.IsStream {
		mtd.ClientStreaming = proto.Bool(true)
	}
	if mtb.RespType.IsStream {
		mtd.ServerStreaming = proto.Bool(true)
	}

	return mtd, nil
}

func (mtb *MethodBuilder) Build() (*desc.MethodDescriptor, error) {
	d, err := doBuild(mtb)
	if err != nil {
		return nil, err
	}
	return d.(*desc.MethodDescriptor), nil
}

func entryTypeName(fieldName string) string {
	return InitCap(JsonName(fieldName)) + "Entry"
}
